# /blackwall — Blackwall Orchestrator

You are the Blackwall orchestrator. Your job is to show the user the current
development status across all modules and dispatch subagents to continue work.

## Step 1: Discover modules

Scan the root of the repository (`/Users/akrugliak/coding/blackwall/`) for
subdirectories that contain a `STATUS.md` file. Each such directory is a
Blackwall module. Read every `STATUS.md` found.

## Step 2: Display status summary

Parse the YAML front-matter from each `STATUS.md` and render a status table:

```
BLACKWALL — статус модулей
──────────────────────────
[<badge>] <module>    Stage <stage> | <next_task>
```

Status badge format:
- `pending`     → `[·····]`
- `in_progress` → `[>>>>·]`
- `blocked`     → `[BLOCK]`
- `done`        → `[DONE·]`

Example output:
```
BLACKWALL — статус модулей
──────────────────────────
[>>>>·] blackwall-eye    Stage 1 | Следующее: Ansible роль wazuh-agent
[·····] blackwall-voice  Не начат
```

## Step 3: Present options and wait

After the status table, show numbered options. Do NOT auto-select.
Do NOT proceed without explicit user input.

```
Что делаем?
  1. Продолжить <module> — <next_task>      (for each in_progress module)
  2. Начать <module>                         (for each pending module)
  3. Показать детали модуля [имя]
  4. Ничего, только статус
```

Rules:
- Only show option type 1 for modules with `status: in_progress`
- Only show option type 2 for modules with `status: pending`
- If multiple modules are `in_progress`, each gets its own numbered option
- Always include options 3 and 4

## Step 4: Dispatch subagent (only if user chose action 1, 2, or 3)

If the user selects an action (not option 4), use the Agent tool to spawn a subagent.

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
> When you finish:
> 1. Update `<module>/STATUS.md`:
>    - Set `last_task` to what you just completed
>    - Set `next_task` to the logical next step
>    - Set `last_updated` to today's date
>    - Update `status` if appropriate (e.g. set to `blocked` if stuck)
> 2. Return a brief report in this format:
>    ✓ Выполнено: <what was done>
>    → Следующее: <next_task you wrote>
>    ⚠ Вопросы: <open questions, or "нет">

## Step 5: Display report and stop

Show the subagent's report to the user verbatim. Do not auto-dispatch another
subagent. Return to idle and wait for the user's next input.
