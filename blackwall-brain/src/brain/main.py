"""Brain's loop: poll the indexer, analyse each new alert, send to Telegram."""
import logging
import sys
import time

from brain import config, telegram
from brain.analysis import PROMPT_VERSION, AnalysisError, analyze
from brain.indexer import IndexerClient, IndexerError, poll_once
from brain.message import format_fallback_message, format_verdict_message
from brain.models import build_model
from brain.state import State

log = logging.getLogger("brain")


def handle(doc_id: str, alert: dict, *, settings, model, send=telegram.send,
           sleep=time.sleep) -> None:
    rule = alert.get("rule", {})
    try:
        a = analyze(alert, model, sleep=sleep)
        text = format_verdict_message(alert, doc_id, a.verdict, settings.dashboard_url,
                                      settings.rule_links)
        log.info("analysed doc=%s alert=%s rule=%s verdict=%s severity=%s tokens=%d/%d "
                 "latency=%.1fs attempts=%d model=%s prompt=%s",
                 doc_id, alert.get("id"), rule.get("id"), a.verdict.verdict, a.verdict.severity,
                 a.result.input_tokens, a.result.output_tokens, a.result.latency_s, a.attempts,
                 model.name, PROMPT_VERSION)
    except AnalysisError as exc:
        text = format_fallback_message(alert, doc_id, str(exc), settings.dashboard_url,
                                       settings.rule_links)
        log.error("analysis failed doc=%s alert=%s rule=%s: %s; sending fallback",
                  doc_id, alert.get("id"), rule.get("id"), exc)
    if send(settings.telegram_bot_token, settings.telegram_chat_id, text):
        log.info("delivered doc=%s alert=%s", doc_id, alert.get("id"))
    else:
        log.error("telegram failed, alert dropped doc=%s alert=%s", doc_id, alert.get("id"))


def run(settings, *, model, indexer, sleep=time.sleep, iterations: int | None = None) -> None:
    state = State.load(settings.state_file, settings.state_max_ids)
    n = 0
    while iterations is None or n < iterations:
        n += 1
        try:
            batch = poll_once(indexer, state, min_level=settings.analysis_threshold,
                              ignore_rule_ids=settings.ignore_rule_ids,
                              max_backfill_s=settings.max_backfill)
        except IndexerError as exc:
            log.warning("indexer poll failed: %s", exc)
            batch = []
        for doc_id, alert in batch:
            handle(doc_id, alert, settings=settings, model=model, sleep=sleep)
            state.mark(doc_id, alert["timestamp"])
            state.save(settings.state_file)
        sleep(settings.poll_interval)


def main() -> None:
    logging.basicConfig(level=logging.INFO, stream=sys.stdout,
                        format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    try:
        settings = config.load()
    except ValueError as exc:
        print(f"brain: configuration error: {exc}", file=sys.stderr)
        sys.exit(2)
    model = build_model(settings.model)
    indexer = IndexerClient(settings.indexer_url, settings.indexer_user,
                            settings.indexer_password, settings.indexer_verify)
    log.info("brain started: model=%s threshold=%d prompt=%s", model.name,
             settings.analysis_threshold, PROMPT_VERSION)
    run(settings, model=model, indexer=indexer)
