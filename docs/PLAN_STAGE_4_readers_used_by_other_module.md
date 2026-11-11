← [Stage 3](PLAN_STAGE_3_generate_then_write_the_rule.md) | Next → [Stage 5](PLAN_STAGE_5_browser_side.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 4 — readers used by other modules

Signatures of every exported reader stay (other modules and apps call them). Bodies read through `m.store` (`ReadOne`, `ReadAll` with `crud.Query`), never `m.db` directly; a `tenantID` parameter different from the module's tenant answers «not found» (`found=false` / empty, `err=nil`). Keep `Charge`, `Due`, `PaidAmount`, `CompleteOnlinePayment` (new signature, stage 4b), and `ReconcilePayments` (called by `Reconcile`). `CreateCounterPayment`, `StartOnlinePayment`, `StartTerminalPayment`, `RefreshTerminalPayment` and `RefundPayment` are deleted: their checks are the rules above, their external work the calls above, and the app creates records (`grep -rn "\.StartOnlinePayment(\|\.StartTerminalPayment(\|\.RefreshTerminalPayment(\|\.RefundPayment(\|\.CreateCounterPayment(" --include=*.go . | grep -v _test` → empty; if a caller outside the module exists, say so in the PR). The `settle`, `settleTerminal` and `close` functions stay as they are.

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
