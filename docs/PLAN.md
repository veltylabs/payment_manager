---
PLAN: "refactor!: payment_manager on the crud-by-declaration framework — Allow, Rules and ExternalCalls in definition.go, persistence by crud, only domain rules in rule files"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> Never edit `definition_gen.go`: a build error there means `definition.go`, a rule file or a call file is wrong; fix that and run `definitionc` again.
> This plan is dispatched in the second wave of modules (D53), after `patient_directory` (the reference) was approved and `externalcalls` + `crud` D58 are published.

# Plan: `payment_manager` on the crud-by-declaration framework

Today 12 operations across three resources (`ResourcePayment`, `ResourcePaymentOnline`, `ResourceRefund`). Terminal, online gateway and refunds call external systems via `ExternalCalls` (D36, D58, D60): `WaitForReply` for interactive pushes and gateway starts, `RetryInBackground` for reconciliation. Online payment is started by a visitor: its request is a separate table the app opens with `AppServer.Public` (D23), since it has a different permission than a counter payment (D18).

Reference implementation of the same migration (read it first, copy its shape): `patient_directory` — `definition.go` (`Definition`, `Allow`, `Rules`), `rule_patient_create.go`/`rule_patient_update.go`/`rule_patient_search.go` (the rules), `store.go`, `readers.go` (signatures kept, bodies on `m.store`).

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go install webtyp.com/definitionc/cmd/definitionc@v0.1.43
go get webtyp.com/model@v0.2.20 webtyp.com/crud@v0.0.14 webtyp.com/orm@v0.12.30 webtyp.com/storage@v0.2.1 webtyp.com/ddl@v0.0.49 webtyp.com/router@v0.4.21 webtyp.com/definitionc@v0.1.43 webtyp.com/view@v0.6.72 webtyp.com/appkit@v0.2.36 webtyp.com/rpc@v0.2.21 webtyp.com/externalcalls@v0.2.0
```
Remove every `replace` from `go.mod`; `go mod tidy`. Before any change check, with `go doc`, that these exist: `model.ModuleDefinition`, `model.Rules` (`model.AddMyBusinessRule`, `model.HaveNoBusinessRule`), `model.ExternalCallDefinition`, `crud.CurrentUser`, `crud.RuleAnswer`, `crud.ApplyAndPublish`, `crud.ApplyWithoutPublishing`, `crud.Topic`, `crud.Loopback`, and that `definitionc` writes a `definition_gen.go` containing `func (m *Module) MountOperations(`. If one is missing, stop and say so in the PR.
Prerequisite of the whole wave: `patient_directory` was executed, reviewed and approved by the owner; it is the reference and the 17 other modules are dispatched together after it.

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

A rule ends with `return crud.ApplyAndPublish(calls...), nil` (saves, announces, and enqueues external calls) or `crud.ApplyWithoutPublishing(calls...)` (saves and enqueues without announcing); a rejection is `return crud.RuleAnswer{}, err`. It queries with `tx`, never with `m.db`. To write another row it calls `tx.Create`/`tx.UpdateFields`. The old `Committed` hook does not exist: external calls are declared in `Definition.ExternalCalls` and orchestrated by `crud`.

## Tables, stores and rules

An action with a rule below is declared `model.AddMyBusinessRule` in the table's `Rules`; every other allowed action is `model.HaveNoBusinessRule`. A `Search` below is the rule of `Read`.

| Table (`<X>Model`) | Store | `Allow` | Rules (`rule_<entity>_<verb>.go`) & ExternalCalls |
|---|---|---|---|
| `Payment` | `orm.TenantStore` | `model.Create \| model.Read \| model.Update` | `Create`: by `method` — counter (checks of `CreateCounterPayment`: amount = `Charges` due, status paid, no external call) or terminal (status `pending`. ExternalCall: `TerminalPush`, `WaitForReply`, `MaxWaitSeconds: 15`); `Update`: only `["status"]` to `checking` → ExternalCall: `TerminalStatus` (`WaitForReply`, `MaxWaitSeconds: 10`); amounts, provider, tokens `ServerOnly` |
| `OnlinePaymentRequest` (**new table** `payment_online_request`) | `orm.TenantStore` | `model.Create \| model.Read` | public (the app lists it in `AppServer.Public` with Create and Read): `Create`: `reference_module`, `reference_id`, return URL; checks of `StartOnlinePayment` before the gateway; creates the `Payment` (method online, `pending`) with `tx.Create` and stores its id. ExternalCall: `GatewayStart` (`WaitForReply`, `MaxWaitSeconds: 15`), enqueued on Create via `crud.ApplyAndPublish(OnlinePaymentRequestGatewayStart{})` |
| `Refund` | `orm.TenantStore` | `model.Create \| model.Read` | `Create`: checks of `RefundPayment` before the gateway (amount ≤ paid); status `pending`. ExternalCall: `GatewayRefund` (`WaitForReply`, `MaxWaitSeconds: 15`), enqueued on Create via `crud.ApplyAndPublish(RefundGatewayRefund{})` |
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
| `refresh_terminal_payment` | `payment.update` with `["status"]` = `checking` |
| `reconcile_payments` | `payment_reconciliation.create` |

## Stage 1 — `definition.go`

- Set `Allow` on each table exactly as in «Tables, stores and rules»; a table not listed there gets no `Allow` (internal).
- Mark `ServerOnly: true`: `tenant_id`, `created_at`, `updated_at`, and every field the server fills (amounts, `status`, `failure`, `provider`, tokens, `redirect_url`, `payment_id`).
- Delete every `*Args` / `*Result` definition that only carried a removed operation (`grep -n "ArgsModel\|ResultModel" model.go` → only the ones this plan keeps).
- New table `OnlinePaymentRequestModel` (name `payment_online_request`): `id`, `tenant_id`, `reference_module`, `reference_id`, `return_url` (`input.Text()`), `payment_id`, `redirect_url`, `status`, `failure`, `created_at` (server fields `ServerOnly`).
- New table `PaymentReconciliationModel` (name `payment_reconciliation`): `id`, `tenant_id`, `status`, `failure`, `reconciled` (`model.Int()`), `actor_id`, `created_at`; `Allow: model.Create | model.Read`.
- Declare `ExternalCalls: []model.ExternalCallDefinition{...}` on:
  - `PaymentModel`:
    - `{Name: "TerminalPush", Mode: model.WaitForReply, MaxWaitSeconds: 15}`
    - `{Name: "TerminalStatus", Mode: model.WaitForReply, MaxWaitSeconds: 10}`
  - `OnlinePaymentRequestModel`:
    - `{Name: "GatewayStart", Mode: model.WaitForReply, MaxWaitSeconds: 15}`
  - `RefundModel`:
    - `{Name: "GatewayRefund", Mode: model.WaitForReply, MaxWaitSeconds: 15}`
  - `PaymentReconciliationModel`:
    - `{Name: "Reconcile", Mode: model.RetryInBackground, MaxAttempts: 3}`
- Add both new tables to `migrate/`. `ResourcePayment`, `ResourcePaymentOnline`, `ResourceRefund` are deleted (each table/read model is its resource).
- **Keep every `*Args` model that an exported reader's signature still uses** (`grep -n "^func (m \*Module) [A-Z][A-Za-z]*([^)]*Args" *.go` lists them): mark each `Wire: true`; delete only the `*Args` no remaining signature uses.
- D52 (the entity is its own filter): a filter model (`*Filter`, `*FilterModel`, an `*ArgsModel` that only carried filter values) is not kept: its fields become fields of the read model or table (`OmitEmpty`), and the rule reads them from its receiver; delete the filter type and its `Wire`/`NewFilter` plumbing.
- Add `var Definition = model.ModuleDefinition{Name: "payment_manager", Entities: []model.Definition{…every table and read model of this plan…}}` and, on every table and read model, `Rules` as «Tables, stores and rules» says (`AddMyBusinessRule` where a rule is named, `HaveNoBusinessRule` for the other allowed actions; a read model: `Read: model.AddMyBusinessRule`). Delete the hand topics (`Topic*`) and `Err*`/resources no code uses any more.

## Stage 2 — `module.go`

`Deps` keeps its fields; add `IDs model.IDGenerator` (required, `depErr("IDs")` when nil) if it is missing.
`Module` gains `store *orm.Store`, built once in `New`: `m.store = orm.TenantStore(db, deps.TenantID, deps.IDs)`.
Delete the hand-written persistence (create/update/upsert/patch/delete/list/get bodies that only move rows) once its rules moved to the rule files: the row-moving bodies of `CreateCounterPayment`, `StartOnlinePayment`, `StartTerminalPayment`, `RefreshTerminalPayment`, `RefundPayment`, `ReconcilePayments`.

## Stage 3 — generate, then write the rules & external calls

1. Delete `ops.go`, `crud.go`, `queries.go`, `model_crud.go`, `model_orm.go` and `definition_crud.go` (those that exist). Run `definitionc`: it writes one `definition_gen.go` (the types, schemas, external call types `PaymentTerminalPush`, `PaymentTerminalStatus`, `OnlinePaymentRequestGatewayStart`, `RefundGatewayRefund`, `PaymentReconciliationReconcile`, and on `Module`, `ModelName()`, `Definition()`, `MountOperations()`) and the skeletons:
   - `rule_<entity>_<verb>.go` for every action declared `AddMyBusinessRule`.
   - `call_<entity>_<call>.go` for every call in `ExternalCalls`.
2. Add `store.go`: for each table of «Tables, stores and rules» that has a Store, `// Store<Entity> is the store of <entity>.` `func (m *Module) Store<Entity>() crud.Store { return m.store }`.
3. Fill each rule skeleton:
   - `Payment.Create`: if method == "counter", checks due, status = "paid", returns `crud.ApplyAndPublish()`. If method == "terminal", status = "pending", returns `crud.ApplyAndPublish(PaymentTerminalPush{})`.
   - `Payment.Update`: if status == "checking", returns `crud.ApplyAndPublish(PaymentTerminalStatus{})`.
   - `OnlinePaymentRequest.Create`: creates `Payment` row (method online, pending) in `tx`, returns `crud.ApplyAndPublish(OnlinePaymentRequestGatewayStart{})`.
   - `Refund.Create`: checks amount <= paid, returns `crud.ApplyAndPublish(RefundGatewayRefund{})`.
   - `PaymentReconciliation.Create`: returns `crud.ApplyAndPublish(PaymentReconciliationReconcile{})`.
   - Rejection: `return crud.RuleAnswer{}, err`.
4. Fill each `call_<entity>_<call>.go` skeleton:
   - `Payment.TerminalPush(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error)`: calls `m.term.Push(...)`. Returns `crud.FieldsToSave{"provider_token": res.Token, "status": res.Status}`, nil.
   - `Payment.TerminalStatus(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error)`: calls `m.term.Status(...)`. Returns `crud.FieldsToSave{"status": res.Status}`, nil.
   - `OnlinePaymentRequest.GatewayStart(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error)`: calls `m.gw.Start(...)`. Updates `redirect_url` and provider token on request and payment rows.
   - `Refund.GatewayRefund(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error)`: calls `m.gw.Refund(...)`. Returns `crud.FieldsToSave{"status": res.Status}`, nil.
   - `PaymentReconciliation.Reconcile(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error)`: calls `m.ReconcilePayments(...)`. Returns count and status.
Delete every `Op*` constant, `res*`/`Resource*` variable, every `op*` handler, every `view.MountSave`/`view.MountDelete`/`reg.Operation` call and the `webtyp.com/router`, `webtyp.com/rpc` imports that only they used (`grep -rn "reg.Operation\|view.MountSave\|view.MountDelete\|router.Context\|rpc.WriteError" --include=*.go . | grep -v _test` → empty).

## Stage 4 — readers used by other modules

Signatures of every exported reader stay (other modules and apps call them). Bodies read through `m.store` (`ReadOne`, `ReadAll` with `crud.Query`), never `m.db` directly; a `tenantID` parameter different from the module's tenant answers «not found» (`found=false` / empty, `err=nil`). Keep `Charge`, `Due`, `PaidAmount`, `CompleteOnlinePayment` (new signature, stage 4b), and `CreateCounterPayment`/`StartOnlinePayment`/`StartTerminalPayment`/`RefreshTerminalPayment`/`RefundPayment`/`ReconcilePayments` as callers that create the same records inside `m.store.Atomic`.

## Stage 4b — online return: identification only, the truth from the gateway (security fix)

**The defect (proven by the test below):** `CompleteOnlinePayment(r OnlineResult)` «settles the result the adapter already verified»
(`module.go:379-395`), but `transbank`'s return route sends «cancelled» (`TBK_TOKEN` + `TBK_ORDEN_COMPRA`) and «expired»
(`TBK_ORDEN_COMPRA` only) **without asking Transbank**. Anyone who knows a pending payment's id can cancel or expire it with one GET.
**The hole:** the method's signature accepts an outcome from its caller. **The class fix:** the outcome can no longer be passed in.

- Delete `OnlineResult` (`ports.go:98-101`); add
  ```go
  // OnlineReturn identifies the payment a provider's return refers to. It carries no outcome: the module asks the gateway.
  type OnlineReturn struct {
  	PaymentId string // the buy order the provider echoes, when it sends one
  	Token     string // the provider token of the transaction, when it sends one
  }
  ```
- `Gateway` gains `Confirm(token string) (GatewayResult, error) // finalizes the transaction when the provider requires it and reports its real
  state; idempotent`. Implement it in `sandbox/sandbox.go` and in every test fake (`grep -rn "func (.*) Start(req paymentmanager.GatewayStart" .`).
- `func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error)`: load the payment by `ret.PaymentId`, else by provider token
  `ret.Token` (both empty → `ErrMissingArgs`); not pending → return it unchanged (idempotent); `ret.Token != "" && ret.Token != p.ProviderToken`
  → `ErrTokenMismatch = fmt.Err("payment_manager: the return's token is not this payment's; nothing was changed")`; then
  `res, err := m.gw.Confirm(p.ProviderToken)` (the stored token, never the request's) and `settle(p, res, false)`. An outcome still
  `OutcomePending` leaves the payment pending (reconciliation expires it later).
- Delete every remaining use of `OnlineResult` (`grep -rn "OnlineResult" .` → empty).

## Stage 5 — browser side (D15, D16)

- `view.go` (`//go:build browser`): every `view.NewCallerLister(caller, view.Ops{…}, newList)` takes
  `func() model.TableSlice { return &<Entity>List{} }` and `view.Ops` has no `Allow` (the screen reads `<Entity>List.Allow()`).
- `module/browser.go`, `module/server.go` (if present): `appkit.ModuleBrowser`/`appkit.BrowserDeps` → `view.ModuleBrowser`/`view.BrowserDeps`.
- Demo: `seed/demo.go` (`//go:build wasm`) returning `view.Demo{Label, Timezone: "America/Santiago", ActingAs: "u1",
  Modules: []crud.Module{mod}, Migrations: <the module's ddl.Step list, or nil if it has no migrations>,
  Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(<what today's demo main passes>) }}`, built with
  `orm.New(mem.New())`, `New(db, Deps{…})` with the same fakes today's demo uses, and `seed.Load` (when the module has one).
  Delete the old demo: any `web/` demo (`test ! -e web`).
- `grep -rn "webtyp.com/layout\|appkit.BrowserDeps\|appkit.ModuleBrowser\|router/loopback" .` → empty, `go.mod` included.

## Stage 6 — tests (`tests/`, through `crud.LoopbackActingAs(nil, "u1", mod)`; rows in `docs/TESTS.md`)

Delete tests of removed code. Operations are `router.Operation{Module: ModelName, Entity: "<entity>", Action: model.<Action>}`, never a string; fixtures follow `crud` v0.0.14 (`reg.AddStoredEntity`, `crud.CurrentUser`). Add `TestMount_MatchesDefinition` (`crud.Mount(reg, nil, nil, mod)` returns nil) and one test that an allowed write publishes a `*crud.Change` on `crud.Topic(ModelName, "<entity>", model.<Action>)`. Required:

| Test | Setup → call → expected |
|---|---|
| `TestCounter_AmountFromCharges` | counter payment with an amount from the client → `ServerOnly` error; without → amount = due |
| `TestTerminal_PushViaExternalCall` | fake terminal records pushes; create terminal payment → external call runs, one push, status from fake |
| `TestTerminal_RefreshAsksStatus` | update `["status"]` = checking → fake `Status` called, final status stored |
| `TestOnline_PublicRequestCreatesPayment` | `payment_online_request_c` without session (public) → pending online payment and redirect URL from external call |
| `TestRefund_NotMoreThanPaid` | refund above the paid amount → field error, gateway not called |
| `TestRefund_GatewayFailureRecorded` | fake gateway fails → refund status `failed` |
| `TestOnline_ForgedCancelIgnored` | pending online payment; fake gateway `Confirm` reports pending; `CompleteOnlinePayment(OnlineReturn{PaymentId: id})` (what a forged `TBK_ORDEN_COMPRA` produces) → payment still pending |
| `TestOnline_ConfirmedByGateway` | fake `Confirm` reports authorized with the right amount → payment paid; reports cancelled → payment cancelled |
| `TestOnline_TokenMismatchRejected` | `OnlineReturn{PaymentId: id, Token: "other"}` → `ErrTokenMismatch`, payment unchanged |
| `TestDemo_Valid` (`//go:build wasm`) | `seed.Demo()` → no error, `Validate()` nil, at least one screen |

## Stage 7 — translations and documentation (Spanish, veltylabs)

`langc sync .` (zero pending, zero reuse warnings). `README.md` (≤150 lines, 4 mandatory sections; «Sin/Con»: hand-written
operations vs `Allow` + rules) with the operations table of the mapping above. `docs/ARCHITECTURE.md`: rules vs persistence.
Translate to Spanish every README, `docs/*`, diagram and code comment still in English; exported symbols without a usage example get one.

## Delivery check

`gotest` passes; `langc sync .` clean; every grep of this plan empty. A tree that does not compile or whose tests did not run is a
failed delivery.
