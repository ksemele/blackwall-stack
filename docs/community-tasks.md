# Community tasks

Work items to make Blackwall usable and attractive for outside users and
contributors. Each task is self-contained: a fresh session can pick one up with
"take task C<n> from docs/community-tasks.md" and needs nothing else.

Two repos are involved:

- **tui**: `ksemele/blackwall`, local checkout `../blackwall-tui`. The k9s-style
  Wazuh TUI. This is the entry point for new users: it works with any Wazuh 4.x,
  no stack required.
- **stack**: `ksemele/blackwall-stack`, this repo. Ansible-deployed Wazuh stack
  for small fleets (Eye, Voice, Brain, Trap, Fang, Scan).

Positioning (agreed 2026-10-16): the bundle-of-known-tools niche is crowded
(Security Onion, wazuh-ansible, SOCFortress). What sets Blackwall apart is
(1) the TUI (no full-screen k9s-style Wazuh client exists) and (2) a light,
opinionated stack for 1–50 hosts with "signal over noise". Lead with the TUI;
the stack and Brain follow.

## Rules for every task

- Repo docs, comments and commits in English. Brief commits, one step each.
- No real hosts, IPs or domains in either repo; demos and examples use the lab
  (`tests/bin/bw-lab up` in the stack repo) or placeholders.
- Anything outward-facing (posting, opening PRs or issues on GitHub, changing
  repo settings, PRs to other people's repos) is drafted first and done only
  after the user says yes in chat.
- When a task is done: tick its box below, add the date, commit.

## Done

- [x] TUI moved to its own repo, stack renamed to `blackwall-stack` (2026-10-16)
- [x] TUI v0.0.1 released: goreleaser, Homebrew cask, release on tag (2026-10-15)
- [x] Both READMEs state the positioning; the TUI README says the stack is not required

## Overview

| ID | Task | Repo | Priority | Depends on |
|---|---|---|---|---|
| C1 | TUI demo GIF | tui | high | — |
| C2 | CONTRIBUTING, SECURITY, templates | tui, stack | high | — |
| C3 | GitHub repo settings | tui, stack | high | — |
| C4 | Starter issues | tui, stack | high | C2 |
| C5 | Hacktoberfest (deadline 2026-10-31) | tui, stack | high, time-boxed | C2, C3, C4 |
| C6 | Tone down "strike back" | stack | medium | — |
| C7 | "Why Blackwall" comparison section | stack | medium | — |
| C8 | TUI release polish (v0.1.0) | tui | medium | C1 |
| C9 | Brain eval set and harness | stack | medium | — |
| C10 | Brain local LLM backend | stack | low | C9 |
| C11 | Catalog and awesome-list submissions | tui | medium | C1, C8 |
| C12 | Launch posts | tui, stack | medium | C1, C2, C3, C8 |

C1, C2, C3, C6, C7, C9 touch different files and can run in parallel sessions.

---

## C1. TUI demo GIF

- [ ] Status

**Repo:** tui. **Why:** the first thing a visitor sees must be the product.
Right now the README opens with text only.

**Do:**
1. Bring the lab up from the stack repo (`tests/bin/bw-lab up`,
   `tests/bin/bw-check all`) so there are agents and alerts to show.
2. Record with [VHS](https://github.com/charmbracelet/vhs): commit the tape as
   `docs/demo.tape`, output `docs/demo.gif`. Script ~20–30 s: agents table →
   drill into one agent → `:alerts` feed → Sessions → quit.
3. Add a `make demo` target that re-renders the GIF from the tape.
4. Put the GIF right under the one-line description at the top of `README.md`.

**Done when:** GIF under ~3 MB renders on the GitHub README, shows only lab
names (`black-*`), and `make demo` reproduces it.

## C2. CONTRIBUTING, SECURITY and templates

- [ ] Status

**Repo:** tui first, then stack. **Why:** neither repo has contributor docs.

**Do (per repo):**
1. `CONTRIBUTING.md`: dev setup (tui: Go version from `go.mod`, `make`, how to
   run against the lab from the stack repo; stack: Docker, `bw-lab`, `bw-check`),
   how to run tests, commit style (short conventional messages), English only,
   no real hosts in examples, where to ask questions (Discussions, once C3 is done).
2. `SECURITY.md`: how to report a vulnerability privately (GitHub private
   vulnerability reporting), what is in scope.
3. `.github/ISSUE_TEMPLATE/` (bug, feature) and `.github/pull_request_template.md`.
   Bug template for tui asks for Wazuh version, OS, `blackwall --version`.
4. Link CONTRIBUTING from the README.

**Done when:** files exist in both repos and are linked from each README.

## C3. GitHub repo settings

- [ ] Status

**Repo:** tui, stack (settings only, via `gh`; see the gh auth note in memory).
**Needs the user's yes before applying** — draft the exact `gh` commands first.

**Do:**
1. Topics. tui: `wazuh`, `tui`, `siem`, `blue-team`, `security`, `bubbletea`,
   `golang`, `terminal`. stack: `wazuh`, `ansible`, `siem`, `blue-team`,
   `security`, `homelab`, `self-hosted`, `nftables`, `auditd`.
2. Enable Discussions (categories: Q&A, Ideas, Show and tell).
3. Labels: `good first issue`, `help wanted`, `hacktoberfest`, `area/<module>`
   for the stack.
4. Enable private vulnerability reporting (for SECURITY.md from C2).
5. Social preview image (1280×640) for the tui repo, e.g. a frame from C1.

**Done when:** `gh repo view --json repositoryTopics,hasDiscussionsEnabled`
shows the change for both repos.

## C4. Starter issues

- [ ] Status

**Repo:** tui, stack. **Why:** newcomers need small, well-described entry points.

**Do:**
1. Collect 5–10 candidates per repo from `*/STATUS.md` (next tasks, Plans),
   the TUI's `docs/ARCHITECTURE.md` and TODOs in code. Good fits: a new TUI
   view or column, a keybinding, a Linux distro in the lab, a Wazuh rule with
   a test, a doc fix.
2. Each issue: context, files to look at, acceptance criteria, how to test,
   labels (`good first issue` or `help wanted`, `area/*`).
3. Write all drafts to one local file outside the repo, show the user, create
   with `gh issue create` only after approval.

**Done when:** issues are open and labelled in both repos.

## C5. Hacktoberfest (time-boxed, ends 2026-10-31)

- [ ] Status

**Repo:** tui, stack. **Why:** the cheapest way to get the first outside PRs.
Skip the task if C2–C4 can't land before ~2026-10-25.

**Do:** add the `hacktoberfest` topic (C3 commands), label the starter issues
(C4) `hacktoberfest`, make sure CONTRIBUTING says how PRs are reviewed. Remove
the topic in November.

## C6. Tone down "strike back"

- [ ] Status

**Repo:** stack. **Why:** "strike back" and "counter-attack" in public docs
scare off companies and careful contributors; hack-back is legally toxic.
Fang in production is automated blocking, which is ordinary active response.

**Do:**
1. Find wording: `grep -rniE 'strike back|counter-?attack|hack-?back' --include='*.md' .`
   (skip `docs/superpowers/`).
2. In public-facing text (README module table, Fang README and STATUS) say
   "active response / automated blocking". Counter-attack becomes one clearly
   marked line: research only, out of scope for production, not shipped.
3. Ask the user whether to keep the Cyberpunk quote at the top of the README
   (it says "strikes back") and the vision line in `CLAUDE.md`. Keep the
   module codenames either way.

**Done when:** no public doc promises counter-attack as a feature.

## C7. "Why Blackwall" section

- [ ] Status

**Repo:** stack. **Why:** visitors will ask "why not Security Onion?".

**Do:** add a short section to `README.md`: who it is for (1–50 Linux hosts,
homelab, small teams), and an honest comparison with Security Onion (full NDR
suite, heavy hardware), plain wazuh-ansible (deploys Wazuh only, no detections
or response) and SOCFortress-style stacks. Check current facts about each
before writing. Mention the TUI as the daily interface.

**Done when:** a section of ≤ 25 lines with a small comparison table.

## C8. TUI release polish (v0.1.0)

- [ ] Status

**Repo:** tui. **Why:** security people check what they install.

**Do:**
1. goreleaser: `.deb` and `.rpm` via nfpms, `checksums.txt`, keyless signing
   with cosign (or GitHub artifact attestations), SBOM.
2. `CHANGELOG.md` (or goreleaser changelog groups) and release notes with the
   C1 GIF.
3. `blackwall --version` prints version, commit, date.
4. Document Linux install in the README (deb, rpm, tarball, verify signature).
5. Tag `v0.1.0` only after the user says so.

**Done when:** a dry run (`goreleaser release --snapshot --clean`) produces all
artifacts, and the README documents them.

## C9. Brain eval set and harness

- [ ] Status

**Repo:** stack (`blackwall-brain/`). Read its `STATUS.md` first.
**Why:** many projects add an LLM to Wazuh triage; almost none publish how
well it works. An open, labelled set of Wazuh alerts with a harness is useful
to everyone and draws ML/LLM contributors.

**Do:** start with brainstorming (design before code). Expected shape:
1. `blackwall-brain/eval/alerts/`: alerts generated in the lab or synthetic,
   sanitized (no real hosts, users, IPs), each with expected labels: true or
   false positive, category, severity. Start with 30–50, covering the Eye
   rules the stack ships.
2. A script that runs Brain's classification over the set and prints
   accuracy per label, cost and latency; model is a parameter.
3. A README for the set: format, how to add a case, license.
4. A `bw-check` or Makefile entry if it fits the existing tooling.

**Done when:** the harness runs against the current Claude model and the
results are recorded in `STATUS.md`.

## C10. Brain local LLM backend

- [ ] Status

**Repo:** stack (`blackwall-brain/`). **Depends on:** C9 (to compare quality).
**Why:** many self-hosters will not send alerts to a cloud API.

**Do:** brainstorm first. Add an optional backend (e.g. Ollama or any
OpenAI-compatible endpoint) selected by a role variable; Claude stays the
default. Run the C9 harness on one or two local models and write the
comparison into `STATUS.md` and the Brain README.

## C11. Catalog and awesome-list submissions

- [ ] Status

**Repo:** tui. **Depends on:** C1 (GIF), ideally C8.
**Every submission is a PR or form to someone else's project: draft, then ask.**

**Candidates (check each list's current rules first):**
- [terminaltrove.com](https://terminaltrove.com) — TUI catalog
- Bubble Tea's "in the wild" list (charmbracelet/bubbletea README)
- rothgar/awesome-tuis
- Wazuh-related awesome lists, blue-team / SOC awesome lists
- awesome-selfhosted (has a minimum project age rule)

**Done when:** a table of submissions with status (drafted, submitted, merged)
is added to this task.

## C12. Launch posts

- [ ] Status

**Repo:** none (drafts live outside the repo). **Depends on:** C1, C2, C3, ideally C8.
**Claude drafts only; the user posts.**

**Draft, each tuned to its audience:**
- Show HN: "k9s for Wazuh" angle, GIF, install one-liner.
- r/Wazuh and the Wazuh community Slack: TUI for existing Wazuh users, ask for feedback.
- r/selfhosted, r/homelab: the light stack for small fleets plus the TUI.
- r/golang, r/commandline: the TUI and how it is built.
- r/blueteamsec: Brain and its eval set (after C9).
- Habr (Russian): longer article, the TUI and the stack design.

Space the posts out over a few weeks; note which ones brought stars, issues
and PRs here.
