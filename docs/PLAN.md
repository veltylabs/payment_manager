---
PLAN: "test: payment_manager domain tests on the crud framework (part 2 of 2: tests, translations, docs)"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST (a `git push` from your sandbox does not reach GitHub). Open it as soon as the first tests pass and keep committing to it.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.
> **Before you submit:** `git status` must be clean and `git diff --stat main` must list the files of the stages. A pull request that carries only go.mod, go.sum or docs/PLAN.md is a failed delivery: `git add -A`, commit and submit again.

# Plan (part 2 of 2): `payment_manager` — the domain tests

Part 1 is merged (v0.4.47): the module is migrated to the crud-by-declaration framework (`definition.go`, `rule_*.go`, `call_*.go`, `timeouts.go`, `store.go`, `ui/`, `seed/demo.go`). Its old tests were deleted because they called removed methods; it has no tests yet. This part writes them (with the queue double that the external calls need), runs the translations and brings the documentation to the templates. `close`, `settle` and `settleTerminal` are kept as they were: do not move them to crud.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```
No `replace` in `go.mod`; no `go get`: the versions are already the right ones. Never edit `definition_gen.go`.

## Rules for the executor

1. **Do not write or summarize a plan:** the stage files are the spec. Start now.
2. **Read files by ranges** (`sed -n 1,80p file`); never one huge `cat`.
3. **A field a test uses is the one in `definition_gen.go`:** `grep -n "type <Entity> struct" -A25 definition_gen.go`.
4. **No python or regex over the source:** edit file by file; `gotest` after each test file.
5. **Deliver early:** submit as soon as the queue double and the first three tests pass, then keep committing.

## Stages

| Stage | File | What |
|---|---|---|
| 6 — tests | [PLAN_STAGE_6_tests.md](PLAN_STAGE_6_tests.md) | the queue double and the domain tests |
| 7 — translations and documentation | [PLAN_STAGE_7_translations_and_documentati.md](PLAN_STAGE_7_translations_and_documentati.md) | `langc sync .`, README, ARCHITECTURE, diagrams |

## Delivery check

`gotest` passes on every destination; `langc sync .` reports no NEW untranslated key (warnings about messages that already existed before this part are listed in the PR description, never fixed here); every test has its row in `docs/TESTS.md` (`| [TestName](../tests/file_test.go) | rule |`). A tree that does not compile or whose tests did not run is a failed delivery.
