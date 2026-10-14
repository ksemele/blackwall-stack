# Blackwall: `last` output parser (sessions)

- Status: done
- Module: blackwall-voice (Sessions tab data source, `tools/blackwall/internal/sessions`)
- Date: 2026-10-14
- Commits: see `git log` ("feat(blackwall): parser for last output (sessions)")

## What was done

- New package `tools/blackwall/internal/sessions`:
  - `Session` struct (User, TTY, From, Login, Logout, Active, End, Raw).
  - `Parse(fullLog string) []Session` returns one entry per `last` line in input order. It skips the `ossec: output:` header, blank lines and the `wtmp begins` footer. Unparseable lines are kept with `Raw` set.
  - `Session.Duration(now)`: now minus login for active sessions, logout minus login for closed ones, 0 when there is no logout time (down, crash, gone) or the line is raw.
  - `Session.IsReboot()`: true for the `reboot` pseudo-session.
- Fixture `testdata/full_log.txt` covers every row shape: active (`still logged in`), closed (`- <iso>`), `down`, `crash`, `gone - no logout`, `reboot ... still running`, a raw line, and the `wtmp begins` footer.

## Verification

- RED: `go test ./internal/sessions/` failed to build (`undefined: Session`, `undefined: Parse`) before the implementation existed.
- GREEN: `go test ./internal/sessions/ -v` passed all three tests (`TestParse_AllShapes`, `TestParse_EmptyAndHeaderOnly`, `TestDuration`).
- `go test -race ./internal/sessions/`: ok.
- `go vet ./...`: clean. `go test ./...`: all packages ok.

## Deviations

- Real-lab check against `eye-sessions.md`: the header line is exactly `ossec: output: 'blackwall-sessions':`, rows match the fixture shapes, and the reboot row ends in `still running`. No row shape missing from the fixture was found, so the fixture was not changed.
- Real rows carry a trailing `  (HH:MM)` duration column. The parser ignores it, since durations are computed from timestamps.

## Open questions

- None for this task. Task 10 consumes `Parse`, `Session.Duration` and `Session.IsReboot` as named.
