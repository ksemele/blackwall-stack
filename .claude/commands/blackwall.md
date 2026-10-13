# /blackwall — Blackwall Orchestrator

## Configuration

orchestrator_model: opus    # Model running this skill (set in Claude Code /model)
subagent_model: haiku       # Model used when dispatching subagents via Agent tool
                            # Override per-session: /blackwall model=sonnet <task>
                            # Options: haiku | sonnet | opus

You are the Blackwall orchestrator. Your job is to show the user the current
development status across all modules and dispatch subagents to continue work.

## Step 1: Discover modules

Scan the root of the repository (the directory containing this `.claude/` folder) for
subdirectories that contain a `STATUS.md` file. Each such directory is a
Blackwall module. Read every `STATUS.md` found.

**Edge case:** If no subdirectories with `STATUS.md` are found, display:
```
BLACKWALL — no modules. Create a module directory with a STATUS.md file (status: pending).
```
and stop.

## Step 2: Display status summary

Parse the YAML front-matter from each `STATUS.md` and render a status table:

```
BLACKWALL — module status
──────────────────────────
[<badge>] <module>    Stage <stage> | <next_task>
```

**Note on pending modules:** For modules with `status: pending`, the `next_task` field may be empty — display `Not started` instead.

Status badge format:
- `pending`     → `[·····]`
- `in_progress` → `[>>>>·]`
- `blocked`     → `[BLOCK]`
- `done`        → `[DONE·]`

Example output:
```
BLACKWALL — module status
──────────────────────────
[>>>>·] blackwall-eye    Stage 1 | Next: wazuh-agent Ansible role
[·····] blackwall-voice  Not started
```

## Step 3: Present options and wait

After the status table, show numbered options. Do NOT auto-select.
Do NOT proceed without explicit user input.

```
What next?
  1. Continue <module> — <next_task>        (for each in_progress module)
  2. Start <module>                          (for each pending module)
  3. Show module details [name]
  4. Nothing, status only
```

Rules:
- Only show option type 1 for modules with `status: in_progress`
- Only show option type 2 for modules with `status: pending`
- If multiple modules are `in_progress`, each gets its own numbered option
- Always include options 3 and 4

**Edge case — all modules done:** If all modules have `status: done`, display:
```
✅ All modules are done.
```
and only offer options 3 and 4.

## Step 4: Dispatch subagent (only if user chose action 1 or 2)

If the user selects action 1 or 2, use the Agent tool to spawn a subagent.
Use `model: haiku` (or the value of `subagent_model` above) when calling Agent.
The user can override this per-session by passing `model=<name>` in the command args.

**Special handling for option 3 (show details):** If user chose option 3, read and display the full content of `<module>/STATUS.md` inline — do not dispatch a subagent.

Construct the subagent prompt as follows:
1. Include full content of `<module>/CLAUDE.md`
2. Include full content of `<module>/STATUS.md`
3. Include the task (from `next_task` field, or user's custom instruction)
4. Include this exact instruction block:

> You are a Blackwall module subagent working on `<module>`.
> The project context is in the CLAUDE.md content above.
> The current module state is in the STATUS.md content above.
> Your task: `<task>`
>
> ## Git rules (apply to every subagent invocation)
> - Commit your work as you go, on the current branch (`claude-dev`). Multiple small commits are fine.
> - Commit messages must be super-brief: one line, conventional-commit style (`feat(scope): …`, `fix(scope): …`, `docs: …`), under ~70 characters, in English. No body, no HEREDOC. Example: `feat(eye): add wazuh-indexer install task`.
> - No AI attribution anywhere: no `Co-Authored-By`, no `Claude-Session`, no "Generated with" lines in commits.
> - Everything written into the repo (docs, STATUS.md, comments, artifacts) is in English.
> - NEVER push. No `git push` under any circumstances. No `--no-verify`, no `--amend` of pushed commits, no force-push.
> - Never create a new branch. Stay on `claude-dev`.
>
> ## Completion artifact (mandatory)
> When you finish (DONE, DONE_WITH_CONCERNS, BLOCKED, or NEEDS_CONTEXT — any terminal status), write a markdown artifact at:
>
>     docs/superpowers/artifacts/<YYYY-MM-DD>-<module-or-plan-slug>/<task-slug>.md
>
> (Create the directory if it doesn't exist. The orchestrator reads these to learn what was done.)
> Artifact format:
>
>     # <task title>
>     **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
>     **Module:** <module>
>     **Date:** <YYYY-MM-DD>
>     **Commits:** <list of SHAs created>
>
>     ## What was done
>     - bullet
>     - bullet
>
>     ## Verification
>     - command + expected output / actual output
>
>     ## Deviations from plan (if any)
>     - what you changed and why
>
>     ## Open questions / concerns
>     - or "none"
>
> ## When you finish
> 1. Write the artifact md (above).
> 2. Update `<module>/STATUS.md`:
>    - Set `last_task` to what you just completed
>    - Set `next_task` to the logical next step
>    - Set `last_updated` to today's date
>    - Update `status` if appropriate (e.g. set to `blocked` if stuck)
> 3. Return a brief report in this format:
>    ✓ Done: <what was done>
>    → Next: <next_task you wrote>
>    📄 Artifact: <path to artifact md>
>    ⚠ Questions: <open questions, or "none">

## Step 5: Display report and stop

Show the subagent's report to the user verbatim. Do not auto-dispatch another
subagent. Return to idle and wait for the user's next input.
