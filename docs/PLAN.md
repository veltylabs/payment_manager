---
PLAN: "refactor!: payment_manager on the crud-by-declaration framework — Allow, Rules and ExternalCalls in definition.go, persistence by crud, only domain rules in rule files"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 18244195220621887859
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST (a `git push` from your sandbox does not reach GitHub). Open it after Stage 3 with whatever compiles and keep committing to it.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.
> **Before you submit:** `git status` must be clean and `git diff --stat main` must list the stage files (definition.go, rule_*.go, call_*.go, store.go, module.go, tests, docs). A pull request that carries only go.mod, go.sum or docs/PLAN.md is a failed delivery: if that is what your submit shows, `git add -A`, commit and submit again.
> Never edit `definition_gen.go`: a build error there means `definition.go`, a rule file or a call file is wrong; fix that and run `definitionc` again.

# Plan: `payment_manager` on the crud-by-declaration framework

Today 12 operations across three resources (`ResourcePayment`, `ResourcePaymentOnline`, `ResourceRefund`). Terminal, online gateway and refunds call external systems via `ExternalCalls` (D36, D58, D60): `WaitForReply` for interactive pushes and gateway starts, `RetryInBackground` for reconciliation. **Waits are configurable (`Deps.Timeouts`, Stage 2); the defaults are 5 s for the machine to accept a charge, 30 s for the customer to pass the card, 10 s for the gateway to answer.** Nothing here ever waits longer for a customer. A machine that does not answer leaves the payment `pending` and the screen offers «pago realizado correctamente» (`payment_manual_confirmation`). Online payment is started by a visitor: its request is a separate table the app opens with `AppServer.Public` (D23), since it has a different permission than a counter payment (D18).

Reference implementation of the same migration (read it first, copy its shape): `patient_directory` — `definition.go` (`Definition`, `Allow`, `Rules`), `rule_patient_create.go`/`rule_patient_update.go`/`rule_patient_search.go` (the rules), `store.go`, `readers.go` (signatures kept, bodies on `m.store`).

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go install webtyp.com/definitionc/cmd/definitionc@v0.1.43
go get webtyp.com/model@v0.2.20 webtyp.com/crud@v0.0.14 webtyp.com/orm@v0.12.33 webtyp.com/storage@v0.2.2 webtyp.com/ddl@v0.0.49 webtyp.com/router@v0.4.22 webtyp.com/definitionc@v0.1.43 webtyp.com/view@v0.6.74 webtyp.com/appkit@v0.2.36 webtyp.com/rpc@v0.2.21 webtyp.com/externalcalls@v0.2.0
```
Remove every `replace` from `go.mod`; `go mod tidy`.
`definitionc` is installed in `$(go env GOPATH)/bin`, which is not on the PATH of a fresh shell: write `export PATH="$PATH:$(go env GOPATH)/bin"` in the SAME command that runs it. **`definitionc` rewrites `go.mod` to older versions on every run: after EVERY run repeat the `go get` line above and `go mod tidy`.** If a module cannot be fetched, stop and say so in the PR; never write a hash by hand.
Before any change check, with `go doc`, that these exist: `model.ModuleDefinition`, `model.Rules` (`model.AddMyBusinessRule`, `model.HaveNoBusinessRule`), `model.ExternalCallDefinition`, `crud.CurrentUser`, `crud.RuleAnswer`, `crud.ApplyAndPublish`, `crud.ApplyWithoutPublishing`, `crud.Topic`, `crud.LoopbackActingAsWithQueue`, and that `definitionc` writes a `definition_gen.go` containing `func (m *Module) MountOperations(`. If one is missing, stop and say so in the PR.

## Lessons of the reference module (`patient_directory`, executed locally)

- Needs `crud` ≥ v0.0.14 (a filter has no required field) and `definitionc` ≥ v0.1.43. `Search(a, filter *X, text, m)`: a text of `""` is normal (the rule runs on every read).
- Running `definitionc` rewrites `go.mod` to older versions: run the `go get` list of «Tools» again after it, then `go mod tidy`; raise stale indirect dependencies (`rbac`, `mcp`, `appkit`) to the published ones.
- In tests, events reach the publisher given to `crud.LoopbackActingAs(pub, "u1", mod)`, not `Deps.Publisher`; a caller double takes `router.Operation`; a `ddl.Execer` double returns `(storage.Result, error)`; a rule that rejects answers `return crud.RuleAnswer{}, err` (assert the refusal and that nothing was stored).
- `langc sync .` can warn that other languages are untranslated: that is the repo's `lang.json`, not this plan.

## The rule of this plan (decisions D28–D50, D58, D60 of the architecture master)

The framework persists; the module declares and writes only domain rules.

| The module no longer writes | Who does it now |
|---|---|
| id, `tenant_id`, `created_at`/`updated_at`, the transaction loop, create/update/delete bodies, «not found», batch atomicity, idempotent retry by id, paging by `After`, `tenant_id` in every query, schema checks repeated from the model | `crud` + `orm.TenantStore` / `orm.GlobalStore` |
| an operation per verb (`list_*`, `save_*`, `upsert_*`, `delete_*`), its handler, `ops.go`, the permission resources and the event topics | `Allow` + `Rules` in `definition.go`; `definitionc` writes `MountOperations`; `crud` names the operation (`router.Operation{Module, Entity, Action}`; on the wire `GET\|POST\|PATCH\|DELETE /api/<module>/<entity>`, in MCP `<module>.<entity>.read\|create\|update\|delete`) and publishes `<module>.<entity>.created\|updated\|deleted` with a `*crud.Change` after the commit |
| an action with its own permission (approve, settle, receive stock…) | a **record created** in its own table, or a **read model** with its own name (D18) |
| external system calls, outbox tables, retry loops | `Definition.ExternalCalls` + `crud.ApplyAndPublish(calls...)`; `crud` records the call within `tx` and orchestrates execution (`WaitForReply`, `RetryInBackground`, `WaitForConfirmation`) |

A module writes: `definition.go` (`var Definition = model.ModuleDefinition{…}` and, per table, `Allow`, `Rules`, and `ExternalCalls`), one `rule_<entity>_<verb>.go` for every action declared `model.AddMyBusinessRule` (created once by `definitionc`; the skeleton does not compile until completed), one `call_<entity>_<call>.go` for every external call declared, and `store.go` (`func (m *Module) Store<Entity>() crud.Store`, one line per table). Every allowed action that has no rule below is declared `model.HaveNoBusinessRule`: `crud` alone checks it. Read the reference first: `patient_directory` (`definition.go`, `rule_patient_*.go`, `store.go`).

| Rule | Form (the receiver is the entity) |
|---|---|
| Create | `func (p *X) Create(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error)` |
| Update | `func (next *X) Update(a crud.CurrentUser, tx crud.Tx, old *X, fields []string, m *Module) (crud.RuleAnswer, error)` (`next` carries only the sent `fields`) |
| Delete | `func (p *X) Delete(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error)` (`p` is the stored row) |
| Search (Read of a table) | `func (p *X) Search(a crud.CurrentUser, filter *X, text string, m *Module) ([]storage.Condition, error)` — AND conditions only; it runs on every read, `text` may be `""`, and `filter` is a record of the entity with only the fields the caller sent (D52: the entity is its own filter) |
| Read (model without a table) | `func (p *X) Read(a crud.CurrentUser, filter *X, args crud.ReadArgs, m *Module) (model.ModelSlice, error)` (`filter` carries the fields the caller sent, D52) |
| External Call | `func (p *X) CallName(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error)` |

A rule ends with `return crud.ApplyAndPublish(calls...), nil` (saves, announces, and enqueues external calls) or `crud.ApplyWithoutPublishing(calls...)` (saves and enqueues without announcing); a rejection is `return crud.RuleAnswer{}, err`. It queries with `tx`, never with `m.db`. To write another row it calls `tx.Create`/`tx.UpdateFields`. The old `Committed` hook does not exist.

**External calls (verified against `crud` v0.0.14).** A rule asks for a call with `return crud.ApplyAndPublish(<Entity><Call>{}), nil`; `definitionc` writes the type `<Entity><Call>` and the skeleton `call_<entity>_<call>.go`. The call is a **method on the stored record** `p`: it **sets fields on `p`** and returns their column names, `p.Status = StatusDone; return crud.Fields("status"), nil` (`crud.FieldsToSave` is `[]string`, never a map; `crud` saves exactly those columns). On failure return `nil, err`: `crud` marks the attempt failed in the queue and the record keeps its old values (a call cannot write a `failed` status; the reason lives in the queue row). A `WaitForReply` call runs inside the request. A `RetryInBackground` call runs later with an **empty `crud.CurrentUser{}`**: it never uses `a.UserID`. A module that declares calls needs a queue: the app passes `appkit.NewCallQueue(q)`; the tests pass the double of Stage 6.


## Tables, stores and rules

An action with a rule below is declared `model.AddMyBusinessRule` in the table's `Rules`; every other allowed action is `model.HaveNoBusinessRule`. A `Search` below is the rule of `Read`.

| Table (`<X>Model`) | Store | `Allow` | Rules (`rule_<entity>_<verb>.go`) & ExternalCalls |
|---|---|---|---|
| `Payment` | `orm.TenantStore` | `model.Create \| model.Read` | `Create`: by `method` — counter (checks of `CreateCounterPayment`: amount = `Charges` due, status paid, no call) or terminal (`m.term == nil` → `ErrTerminalUnavailable`; checks of `StartTerminalPayment`; status `pending`; `return crud.ApplyAndPublish(PaymentTerminalPush{}, PaymentTerminalStatus{}), nil`). ExternalCalls: `TerminalPush` (`WaitForReply`, cap `TerminalPushCapSeconds` = 10), `TerminalStatus` (`RetryInBackground`, `MaxAttempts: 20`). No `Update` (the old `checking` trick is deleted); amounts, provider, tokens `ServerOnly` |
| `PaymentManualConfirmation` (**new table** `payment_manual_confirmation`) | `orm.TenantStore` | `model.Create \| model.Read` | own permission (the cashier says «paid» when the machine could not be reached): `Create`: `payment_id` must be a `pending` terminal payment (else `ErrPaymentNotPending`), `note` required (field error), `actor_id` = `a.UserID`. ExternalCall: `Confirm` (`WaitForReply`, cap 10) closes the payment as paid |
| `OnlinePaymentRequest` (**new table** `payment_online_request`) | `orm.TenantStore` | `model.Create \| model.Read` | public (the app lists it in `AppServer.Public` with Create and Read): `Create`: `reference_module`, `reference_id`, return URL; checks of `StartOnlinePayment` before the gateway; creates the `Payment` (method online, `pending`, same id as the request) with `tx.Create`. ExternalCall: `GatewayStart` (`WaitForReply`, cap `OnlineStartCapSeconds` = 15; the response of the create carries `redirect_url`, `token`, `token_field` because the call sets them on the record, `crud` v0.0.14 `stored_entity_create.go`), enqueued on Create via `crud.ApplyAndPublish(OnlinePaymentRequestGatewayStart{})` |
| `Refund` | `orm.TenantStore` | `model.Create \| model.Read` | `Create`: checks of `RefundPayment` before the gateway (amount ≤ paid); status `pending`. ExternalCall: `GatewayRefund` (`WaitForReply`, cap 15), enqueued on Create via `crud.ApplyAndPublish(RefundGatewayRefund{})` |
| `PaymentReconciliation` (**new table** `payment_reconciliation`) | `orm.TenantStore` | `model.Create \| model.Read` | `Create`: status `pending`, `actor_id`. ExternalCall: `Reconcile` (`RetryInBackground`, `MaxAttempts: 3`), enqueued via `crud.ApplyAndPublish(PaymentReconciliationReconcile{})` |
| `PaymentCounter` | — (internal, no `Allow`) | — | numbering used by counter payments |

## Read models (D18 — no table, `Allow: model.Read` only, `Rules.Read: model.AddMyBusinessRule`)

| Model | Filter | Answered by |
|---|---|---|
| `ChargeResult` (existing; `Allow: model.Read`) — public by the app | `ChargeFilter{reference_module, reference_id string}` | `Charge(...)` |
| `PaymentStatusResult` (existing) — public by the app | `PaymentStatusFilter{id string}` | status of one payment by id |

A read model is a `model.Definition` **without** `DB:` marks, with `Allow: model.Read` and `Rules: model.Rules{Read: model.AddMyBusinessRule}`, listed in `Definition.Entities`; `definitionc` mounts it and creates its `rule_<model name>_read.go`. Its permission resource is its own `ModelName`: list it in the README permissions table (roles must be granted it).

## Operation mapping (every operation of today has exactly one row)

«After» is `router.Operation{Module, Entity, Action}` written `<entity>.<verb>`; the wire form is `crud`'s (REST `/api/<module>/<entity>` by method, MCP `<module>.<entity>.<verb>`).

| Today | After |
|---|---|
| `get_charge` | read model `ChargeResult` |
| `create_counter_payment` | `payment.create` with `method: "counter"` |
| `start_online_payment` | `payment_online_request.create` (public) |
| `get_payment_status` | read model `PaymentStatusResult` (public) |
| `get_payment / list_payments` | `payment.read` |
| `list_payments_by_reference` | `payment.read` with filter `{reference_module, reference_id}` |
| `refund_payment` | `refund.create` |
| `list_refunds` | `refund.read` |
| `start_terminal_payment` | `payment.create` with `method: "terminal"` |
| `refresh_terminal_payment` | **automatic**: the `TerminalStatus` polling call (no operation); the cashier's manual way out is `payment_manual_confirmation.create` |
| `reconcile_payments` | `payment_reconciliation.create` |

## Rules learned from the failed sessions of this wave (read before starting)

1. **Do not write, summarize or iterate on a plan.** This index and the stage files are the finished spec. Start Stage 1 now.
2. **Read files by ranges** (`sed -n 1,80p file`, then `sed -n 81,160p file`); never one huge `cat`: its output is cut.
3. **A field a rule uses is the one in `definition_gen.go`:** `grep -n "type <Entity> struct" -A25 definition_gen.go`. `ServerOnly` only blocks the CLIENT: a rule may set any field of its receiver and of rows it creates with `tx.Create`. A field that does not exist is a question for the PR (name it), never an invention.
4. **No python or regex over the source:** edit file by file; `gotest` after each stage.

## Stages (do them in order; commit after each)

| Stage | File |
|---|---|
| 1 — `definition.go` | [PLAN_STAGE_1_definition_go.md](PLAN_STAGE_1_definition_go.md) |
| 2 — `module.go` | [PLAN_STAGE_2_module_go.md](PLAN_STAGE_2_module_go.md) |
| 3 — generate, then write the rules & external calls | [PLAN_STAGE_3_generate_then_write_the_rule.md](PLAN_STAGE_3_generate_then_write_the_rule.md) |
| 4 — readers used by other modules | [PLAN_STAGE_4_readers_used_by_other_module.md](PLAN_STAGE_4_readers_used_by_other_module.md) |
| 5 — browser side (D15, D16) | [PLAN_STAGE_5_browser_side.md](PLAN_STAGE_5_browser_side.md) |
| 6 — tests (`tests/`, through `crud.LoopbackActingAsWithQueue(nil, "u1", &callQueue{}, mod)`; rows in `docs/TESTS.md`) | [PLAN_STAGE_6_tests.md](PLAN_STAGE_6_tests.md) |
| 7 — translations and documentation (Spanish, veltylabs) | [PLAN_STAGE_7_translations_and_documentati.md](PLAN_STAGE_7_translations_and_documentati.md) |

## Delivery check

`gotest` passes; `langc sync .` clean; every grep of this plan empty. A tree that does not compile or whose tests did not run is a
failed delivery.
