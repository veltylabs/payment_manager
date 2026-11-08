---
PLAN: "refactor!: payment_manager on crud tables (D19/D20) — Allow in the model, persistence by the framework, only domain rules"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> Never edit `model_orm.go` or `model_crud.go`: a build error there means the hand-written code is wrong.

# Plan: `payment_manager` on crud tables

Today 12 operations across three resources (`ResourcePayment`, `ResourcePaymentOnline`, `ResourceRefund`). Terminal, online gateway and refunds call external systems: records with an outbox (D25). Online payment is started by a visitor: its request is a separate table the app opens with `AppServer.Public` (D23), since it has a different permission than a counter payment (D18).

Reference implementation of the same migration (read it first, copy its shape):
`patient_directory` — `model.go` (`Allow`), `ops.go` (`MountPatient`), `rules.go` (hooks), `readers.go` (signatures kept, bodies on `m.store`).

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go install webtyp.com/ormc/cmd/ormc@latest
go get webtyp.com/model@latest webtyp.com/crud@latest webtyp.com/orm@latest webtyp.com/view@latest webtyp.com/appkit@latest
```
These versions must contain `Definition.Allow` and `model.TableSlice` (model), `crud.Table`/`crud.Tx`/`crud.Transactional` (crud),
`orm.TenantStore`/`orm.GlobalStore` returning `*orm.Store` (orm), `ormc` generating `Mount<Entity>` + `<Entity>Rules`, and `view`
whose `NewCallerLister` takes `func() model.TableSlice`. If `go doc webtyp.com/orm TenantStore` fails or `ormc` does not write
`func Mount`, stop and say so in the PR. Remove every `replace` from `go.mod`; `go mod tidy`; run `ormc`.

## The rule of this plan (D19/D20/D21 of the architecture master)

The framework persists; the module declares and writes only domain rules.

| The module no longer writes | Who does it now |
|---|---|
| id, `tenant_id`, `created_at`/`updated_at`, the transaction loop, `db.Create`/`db.Update`/`db.Delete`, «not found», batch atomicity, idempotent retry by id, paging by `After`, `tenant_id` in every query, schema checks repeated from the model | `crud` + `orm.TenantStore` / `orm.GlobalStore` |
| an operation per verb (`list_*`, `save_*`, `upsert_*`, `delete_*`) and its handler | `Allow` in the table's `model.Definition` + the generated `Mount<Entity>` |
| an action with its own permission (approve, settle, receive stock…) | a **record created** in its own table, or a **read model** with its own name (D18) |

A module writes: `Allow` per table; `ops.go` with one `Mount<Entity>(reg, store, <Entity>Rules{…})` per offered table and one
`reg.Read(…)` per read model; `rules.go` with the hooks (`Create(c, tx, rec)`, `Update(c, tx, old, next, fields)`,
`Delete(c, tx, rec)`, `Search(c, text)` returning AND-only conditions, `Committed(c, action, rec, fields)` for events after commit).
A hook changes only the columns in `fields` on Update; to write another row it calls `tx.Create`/`tx.UpdateFields`.

## Tables, stores and rules

| Table (`<X>Model`) | Store | `Allow` | Hooks (`rules.go`) — where their rules come from today |
|---|---|---|---|
| `Payment` | `orm.TenantStore` | `model.Create \| model.Read \| model.Update` | `Create`: by `method` — counter (checks of `CreateCounterPayment`: amount = `Charges` due, status paid, no external call) or terminal (status `pending`; `Committed`: `Terminal.Push`, store token/status); `Update`: only `["status"]` to `checking` → `Committed` asks `Terminal.Status`/`Gateway.Status` and writes the outcome (refresh); amounts, provider, tokens `ServerOnly`. **Outbox (D25):** the record is created with status `pending`; `Committed` calls the external system and then writes the outcome with `m.store.Atomic(func(tx crud.Tx) error { return tx.UpdateFields(rec, id, []string{"status", "failure", …}) })` — status constants `StatusPending = "pending"`, `StatusDone = "done"`, `StatusFailed = "failed"`. A retry is an Update of `status` back to `pending` (its `Committed` calls again). Never call the external system inside a `Create`/`Update` hook. |
| `OnlinePaymentRequest (**new table** `payment_online_request`)` | `orm.TenantStore` | `model.Create \| model.Read` | public (the app lists it in `AppServer.Public` with Create and Read): `Create`: `reference_module`, `reference_id`, return URL; checks of `StartOnlinePayment` before the gateway; creates the `Payment` (method online, `pending`) with `tx.Create` and stores its id; `Committed`: `Gateway.Start`, store the redirect URL/token on both rows |
| `Refund` | `orm.TenantStore` | `model.Create \| model.Read` | `Create`: checks of `RefundPayment` before the gateway (amount ≤ paid); status `pending`; `Committed`: `Gateway.Refund`, write status |
| `PaymentCounter` | — (internal, no `Allow`) | — | numbering used by counter payments |

## Read models (D18 — `crud.Registry.Read`, no table, `Allow: model.Read` only)

| Model | Filter | Answered by |
|---|---|---|
| `ChargeResult` (existing; `Allow: model.Read`) — public by the app | `ChargeFilter{reference_module, reference_id string}` | `Charge(...)` |
| `PaymentStatusResult` (existing) — public by the app | `PaymentStatusFilter{id string}` | status of one payment by id |

A read model is a `model.Definition` **without** `DB:` marks and with `Allow: model.Read`; `ormc` gives it `Allow()` and no
`Mount`. Its permission resource is its own `ModelName`: list it in the README permissions table (roles must be granted it).

## Operation mapping (every operation of today has exactly one row)

| Today | After |
|---|---|
| `get_charge` | read model `ChargeResult` |
| `create_counter_payment` | `payment_c` with `method: "counter"` |
| `start_online_payment` | `payment_online_request_c` (public) |
| `get_payment_status` | read model `PaymentStatusResult` (public) |
| `get_payment / list_payments` | `payment_r` |
| `list_payments_by_reference` | `payment_r` with filter `{reference_module, reference_id}` |
| `refund_payment` | `refund_c` |
| `list_refunds` | `refund_r` |
| `start_terminal_payment` | `payment_c` with `method: "terminal"` |
| `refresh_terminal_payment` | `payment_u` with `["status"]` = `checking` |
| `reconcile_payments` | `payment_reconciliation_c` (new table, outbox: `Committed` runs `ReconcilePayments` and stores the count) |

## Stage 1 — `model.go`

- Set `Allow` on each table exactly as in «Tables, stores and rules»; a table not listed there gets no `Allow` (internal).
- Mark `ServerOnly: true`: `tenant_id`, `created_at`, `updated_at`, and every field the server fills (amounts, `status`, `failure`, `provider`, tokens, `redirect_url`, `payment_id`).
- Delete every `*Args` / `*Result` definition that only carried a removed operation
  (`grep -n "ArgsModel\|ResultModel" model.go` → only the ones this plan keeps).
- New table `OnlinePaymentRequestModel` (name `payment_online_request`): `id`, `tenant_id`, `reference_module`, `reference_id`, `return_url` (`input.Text()`), `payment_id`, `redirect_url`, `status`, `failure`, `created_at` (server fields `ServerOnly`).
- New table `PaymentReconciliationModel` (name `payment_reconciliation`): `id`, `tenant_id`, `status`, `failure`, `reconciled` (`model.Int()`), `actor_id`, `created_at`; `Allow: model.Create | model.Read`; `Committed` runs `ReconcilePayments(tenant, now)`.
- Add both to `migrate/`. `ResourcePayment`, `ResourcePaymentOnline`, `ResourceRefund` are deleted (each table/read model is its resource).
- **Keep every `*Args` model that an exported reader's signature still uses** (`grep -n "^func (m \*Module) [A-Z][A-Za-z]*([^)]*Args" *.go` lists them): mark each `Wire: true`; delete only the `*Args` no remaining signature uses.
- Run `ormc`: `model_crud.go` has one `func Mount<Entity>(` per table with `Allow`.

## Stage 2 — `module.go`

`Deps` keeps its fields; add `IDs model.IDGenerator` (required, `depErr("IDs")` when nil) if it is missing.
`Module` gains `store *orm.Store`, built once in `New`: `m.store = orm.TenantStore(db, deps.TenantID, deps.IDs)`.
Delete the hand-written persistence (create/update/upsert/patch/delete/list/get bodies that only move rows) once its rules moved to
`rules.go`: the row-moving bodies of `CreateCounterPayment`, `StartOnlinePayment`, `StartTerminalPayment`, `RefreshTerminalPayment`, `RefundPayment`, `ReconcilePayments` (external calls move to `Committed`).

## Stage 3 — `ops.go` and `rules.go`

```go
const ModelName = "payment_manager"

func (m *Module) ModelName() string { return ModelName }

var _ crud.Module = (*Module)(nil)

func (m *Module) MountOperations(reg crud.Registry) {
	MountPayment(reg, m.store, PaymentRules{Create: m.checkPayment, Update: m.refreshPayment, Committed: m.paymentCommitted})
	MountOnlinePaymentRequest(reg, m.store, OnlinePaymentRequestRules{Create: m.startOnline, Committed: m.onlineStarted})
	MountRefund(reg, m.store, RefundRules{Create: m.checkRefund, Committed: m.refundCommitted})
	MountPaymentReconciliation(reg, m.store, PaymentReconciliationRules{Create: m.checkReconciliation, Committed: m.reconcile})
	reg.Read(&ChargeResult{}, func() model.Model { return &ChargeFilter{} }, m.readCharge)
	reg.Read(&PaymentStatusResult{}, func() model.Model { return &PaymentStatusFilter{} }, m.readPaymentStatus)
}
```
`rules.go` holds every hook named above. **Outbox (D25):** the record is created with status `pending`; `Committed` calls the external system and then writes the outcome with `m.store.Atomic(func(tx crud.Tx) error { return tx.UpdateFields(rec, id, []string{"status", "failure", …}) })` — status constants `StatusPending = "pending"`, `StatusDone = "done"`, `StatusFailed = "failed"`. A retry is an Update of `status` back to `pending` (its `Committed` calls again). Never call the external system inside a `Create`/`Update` hook. `CompleteOnlinePayment` changes as stage 4b says (security fix) and writes through `m.store`.
Delete every `Op*` constant, `res*`/`Resource*` variable that named a removed operation, every `op*` handler, every
`view.MountSave`/`view.MountDelete`/`reg.Operation` call and the `webtyp.com/router`, `webtyp.com/rpc` imports
(`grep -rn "reg.Operation\|view.MountSave\|view.MountDelete\|router.Context\|rpc.WriteError" --include=*.go . | grep -v _test` → empty).

## Stage 4 — readers used by other modules

Signatures of every exported reader stay (other modules and apps call them). Bodies read through `m.store` (`ReadOne`, `ReadAll` with `crud.Query`), never `m.db` directly; a `tenantID` parameter different from the module's tenant answers «not found» (`found=false` / empty, `err=nil`). Keep `Charge`, `Due`, `PaidAmount`, `CompleteOnlinePayment` (new signature, stage 4b), and `CreateCounterPayment`/`StartOnlinePayment`/`StartTerminalPayment`/`RefreshTerminalPayment`/`RefundPayment`/`ReconcilePayments` as callers that create the same records inside `m.store.Atomic` (external calls only after commit).

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

## Stage 6 — tests (`tests/`, through `crud.LoopbackActingAs("u1", mod)`; rows in `docs/TESTS.md`)

Delete tests of removed code. Operation names: `crud.Op(&<Entity>{}, model.<Action>)` qualified with `ModelName + "."`. Required:

| Test | Setup → call → expected |
|---|---|
| `TestCounter_AmountFromCharges` | counter payment with an amount from the client → `ServerOnly` error; without → amount = due |
| `TestTerminal_PushAfterCommit` | fake terminal records pushes; create terminal payment → one push, status from the fake |
| `TestTerminal_RefreshAsksStatus` | update `["status"]` = checking → fake `Status` called, final status stored |
| `TestOnline_PublicRequestCreatesPayment` | `payment_online_request_c` without session (public) → a pending online payment and the redirect URL after `Gateway.Start` |
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
