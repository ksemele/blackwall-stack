"""Replay snapshots through Brain's real analysis and score the verdicts.

    python -m brain.eval.run [--backend B] [--model M] [--options JSON] [--max-cost USD]

API key from BRAIN_MODEL_API_KEY (falls back to BW_CLAUDE_API_KEY). Calls a
paid API: run by hand, never in CI.
"""
import argparse
import json
import os
import sys
import time
from dataclasses import asdict
from datetime import date

from brain.analysis import PROMPT_VERSION, AnalysisError, analyze
from brain.eval.capture import EVAL_DIR
from brain.eval.score import Outcome, estimate_cost, score
from brain.eval.snapshots import EVAL_SET_VERSION, load_cases
from brain.models import ModelSettings, build_model


def _report(s: dict, cost: float | None) -> str:
    def pct(x):
        return "n/a" if x is None else f"{x * 100:.0f}%"

    lines = [
        f"Missed threats (real threat called noise): {len(s['missed_threats'])}",
        *[f"  - {m}" for m in s["missed_threats"]],
        f"Threats not called (uncertain): {s['threats_not_called']}",
        f"Verdict accuracy: {pct(s['verdict']['accuracy'])}  (n={s['n']})",
        f"Severity accuracy: {pct(s['severity']['accuracy'])}",
        f"Category accuracy: {pct(s['category']['accuracy'])}",
        f"Uncertain: {pct(s['uncertain_rate'])}   Invalid output: {pct(s['invalid_rate'])}",
        f"Decidable without context (n={s['subset_without_context_need']['n']}): verdict "
        f"{pct(s['subset_without_context_need']['verdict']['accuracy'])}",
        f"At threshold (n={s['subset_at_threshold']['n']}): verdict "
        f"{pct(s['subset_at_threshold']['verdict']['accuracy'])}",
        f"Tokens: {s['tokens']['input']} in / {s['tokens']['output']} out   "
        f"Cost: {'n/a' if cost is None else f'${cost:.4f}'}",
        f"Latency: p50 {s['latency']['p50'] or 0:.1f}s  p95 {s['latency']['p95'] or 0:.1f}s",
    ]
    if s["needs"]:
        lines.append("Most wanted data: " + "; ".join(f"{k} ({v})" for k, v in s["needs"]))
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(prog="python -m brain.eval.run")
    p.add_argument("--snapshots", default=os.path.join(EVAL_DIR, "snapshots"))
    p.add_argument("--out", default=os.path.join(EVAL_DIR, "results"))
    p.add_argument("--backend", default="anthropic", choices=["anthropic", "openai_compat"])
    p.add_argument("--model", default="claude-haiku-5-5")
    p.add_argument("--base-url", default="")
    p.add_argument("--options", default='{"effort": "low"}', type=json.loads)
    p.add_argument("--threshold", type=int, default=10)
    p.add_argument("--max-cost", type=float, default=None)
    args = p.parse_args(argv)

    prices = json.load(open(os.path.join(EVAL_DIR, "prices.json")))
    if args.max_cost is not None and args.model not in prices:
        print(f"warning: no price for {args.model}; --max-cost is not enforced")
    key = os.environ.get("BRAIN_MODEL_API_KEY") or os.environ.get("BW_CLAUDE_API_KEY", "")
    model = build_model(ModelSettings(args.backend, args.model, key, args.base_url,
                                      options=args.options))
    cases = load_cases(args.snapshots)

    outcomes, stopped = [], None
    for i, c in enumerate(cases, 1):
        try:
            a = analyze(c.alert, model, sleep=time.sleep)
            o = Outcome(c, a.verdict, "", a.result.input_tokens, a.result.output_tokens,
                        a.result.latency_s)
        except AnalysisError as exc:
            o = Outcome(c, None, str(exc), 0, 0, 0.0)
        except Exception as exc:  # a backend bug must not lose a paid run
            o = Outcome(c, None, f"{type(exc).__name__}: {exc}", 0, 0, 0.0)
        outcomes.append(o)
        print(f"[{i}/{len(cases)}] {c.scenario}/{c.doc_id}: "
              f"{o.verdict.verdict if o.verdict else 'invalid'}", file=sys.stderr)
        spent = estimate_cost(args.model, sum(x.input_tokens for x in outcomes),
                              sum(x.output_tokens for x in outcomes), prices)
        if args.max_cost is not None and spent is not None and spent > args.max_cost:
            stopped = "max_cost"
            print(f"stopping: estimated ${spent:.4f} > --max-cost {args.max_cost}")
            break

    s = score(outcomes, args.threshold)
    cost = estimate_cost(args.model, s["tokens"]["input"], s["tokens"]["output"], prices)
    print(_report(s, cost))
    result = {
        "model": args.model, "backend": args.backend, "options": args.options,
        "prompt_version": PROMPT_VERSION, "eval_set_version": EVAL_SET_VERSION,
        "date": date.today().isoformat(), "cost_usd": cost, "score": s,
        "per_case": [{"scenario": o.case.scenario, "doc_id": o.case.doc_id,
                      "expected": asdict(o.case.expected),
                      "got": asdict(o.verdict) if o.verdict else None, "error": o.error}
                     for o in outcomes],
    }
    if stopped:
        result["stopped"] = stopped
    os.makedirs(args.out, exist_ok=True)
    effort = args.options.get("effort")
    name = f"{date.today().isoformat()}-{args.backend}-{args.model}" + \
           (f"-{effort}" if effort and effort != "low" else "") + ".json"
    with open(os.path.join(args.out, name), "w") as f:
        json.dump(result, f, indent=2, sort_keys=True, ensure_ascii=False)
        f.write("\n")
    print(f"wrote {os.path.join(args.out, name)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
