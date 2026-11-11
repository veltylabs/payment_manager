---
PLAN: "chore: payment_manager part 2b (reuse warnings, translations, Spanish docs)"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST. Submit after EACH stage that compiles and keep committing.
> **Never ask a yes/no question about progress**; ask only when something in this plan is wrong, and name it.
> **Before you submit:** `git status` must be clean and `git diff --stat main` must list the files of the plan. A pull request that carries only go.mod, go.sum or docs/PLAN.md is a failed delivery.

# Plan: `payment_manager` part 2b — text debt only (no logic change)

Part 2a is merged (v0.4.48). This plan touches only error/label text, `lang.json` and docs. No new behaviour, no `definition_gen.go` edit.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```
No `go get`, no `replace`. Read files by ranges (`sed -n 1,80p file`).

## Stage 1 — remove the reuse warnings

Run `langc sync .`. It reports (verbatim):

```
'payment is not a card terminal payment' and 'payment is not paid' share 'payment is not' — extract it: fmt.Err("payment is not", {a card terminal payment | paid})
'payment not found' and 'refund not found' share 'not found' — extract it: fmt.Err("not found", {payment | refund})
'the reference has nothing to pay' and 'the reference is already paid' share 'the reference' — extract it: fmt.Err("the reference", {has nothing to pay | is already paid})
```

Find the constants: `grep -n "not a card terminal payment\|is not paid\|not found\|has nothing to pay\|already paid" errors.go`. For each pair, rebuild the two messages from the shared prefix plus a differing word passed as data, exactly the way the other composed errors of `errors.go` are written (read the file first and copy that pattern). The error VALUES that tests compare (`errors.Is`/equality) must keep working: run `gotest` after each pair.

## Stage 2 — translate

`langc sync .` again, then fill every empty value and every value that still equals the English text (the "placeholder token x" error) in `lang.json`, for every language column. Finish only when `langc sync .` prints 0 untranslated keys, 0 reuse warnings, 0 identical values.

## Stage 3 — Spanish docs

Every `README.md`, `docs/*.md` and `docs/diagrams/*.md` still in English becomes Spanish for a reader with little technical background (same content, same structure; README ≤ 150 lines with sections `¿Por qué existe?`, `Antes de comenzar`, `Guía paso a paso`, `Mapa de documentación`; `docs/ARCHITECTURE.md` with its 4 sections; each diagram: context, `flowchart TD`, numbered walkthrough). Translate every code comment still in English and give each exported symbol's comment a short `// Ejemplo:` line when it lacks one. Do not change code.

Add `docs/diagrams/PLACEMENT.md` (context, `flowchart TD`, walkthrough): what this module imports (read `go.mod`) and who imports it (`grep -l 'veltylabs/payment_manager' ../*/go.mod ../../*/go.mod`).

## Delivery check

`gotest` passes (a tree that does not compile is a failed delivery); `langc sync .` reports 0/0/0; `git diff --stat main` lists `errors.go`, `lang.json`, README and docs.

| Stage | Files |
|---|---|
| 1 | `errors.go`, call sites |
| 2 | `lang.json` |
| 3 | `README.md`, `docs/*.md`, `docs/diagrams/*.md`, comments |
