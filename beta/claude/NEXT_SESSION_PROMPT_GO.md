# Prompt: Drone service Go port, continue from Phase 1

Read these first, in order:
1. `/home/alex/DSETraining/beta/claude/HANDOFF_GO_2026-10-29.md`: current state, rules, plan, contract, the list of Go
   syntax already introduced.
2. `/home/alex/DSETraining/alpha/claude/COACHING_PROMPT.md`: my coaching rules. Use its **Phase 2 (coaching mode)**:
   help ladder, LEARN / BORROW / SKIP verdicts, "check" means review. Its Phase 1 cleanup is long done; skip it.
3. The Go code in `/home/alex/DSETraining/beta`, and the Python files in `/home/alex/DSETraining/alpha` that the next
   phase ports (Phase 1: `domain/models.py`, `domain/updates.py`, `domain/geo.py`, `api/schemas.py`).

Context: I'm porting my Python drone service (pymavlink + FastAPI + SQLite, verified on ArduPilot SITL) to Go to learn Go.
I have zero Go experience before this week but strong CS fundamentals and I know C++ well. Phase 0 (toolchain, module,
config package) is done.

## How to run this session

- **Every task starts with:** a one-line verdict (LEARN / BORROW / SKIP-MINIMAL), Level 0 (goal, concept, questions,
  files, what "done" looks like), then a **Go syntax primer in chat**: small standalone snippets (not my solution) that
  explain every Go construct I haven't seen yet: what it is, what it does, and the C++/Python equivalent. Skip what's
  already in the handoff's "Introduced so far" list.
- I type the code myself. Don't edit my files in `beta/` unless I say "do it for me". If I say "give me the code to
  type", show the full solution in chat after verifying it on a scratch copy, with notes on what's new.
- When I say **"check"**, run my code (`go run`, `go vet ./...`, `gofmt -l .`, and drive it with fakes or test input),
  show me the real output, and use "you have / needed / why" for anything wrong. Hints, not fixes, unless I ask for more.
- Go up the help ladder only when I ask. Explain the *why*, especially for concurrency.
- Keep the HTTP contract and SQLite schema in the handoff identical, so the React app and compose file work unchanged.
- At the end of the session, write the next handoff into `beta/claude/` (including the updated "Introduced so far" list)
  and a prompt for the next chat.

## Start
Ask me the open decisions from the handoff in one short message (python-mvp tag, `cmd` params, git init in beta), then start **Phase 1: domain types + geo distance** with the verdict, Level 0 and the syntax
primer.
