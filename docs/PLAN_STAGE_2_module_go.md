← [Stage 1](PLAN_STAGE_1_definition_go.md) | Next → [Stage 3](PLAN_STAGE_3_generate_then_write_the_rule.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 2 — `module.go`

`Deps` keeps its fields; add `IDs model.IDGenerator` (required, `depErr("IDs")` when nil) if it is missing.
`timeouts.go` (new): the waits, configurable, with defaults and an upper bound each (crud's declared cap):

```go
const (
	DefaultTerminalPushSeconds      = 5  // the machine must accept the charge or the cashier is told «no connection»
	DefaultTerminalPayWindowSeconds = 30 // how long the customer has to pass the card before the payment is offered for manual confirmation
	DefaultOnlineStartSeconds       = 10 // the gateway must answer with the payment link
	TerminalPushCapSeconds          = 10 // Definition caps: Timeouts above them are refused by New
	OnlineStartCapSeconds           = 15
)

// Timeouts are the waits of the module. A zero field takes its default. Example: Deps{Timeouts: Timeouts{TerminalPayWindowSeconds: 60}}
type Timeouts struct {
	TerminalPushSeconds      int64
	TerminalPayWindowSeconds int64
	OnlineStartSeconds       int64
}

func (t Timeouts) withDefaults() (Timeouts, error) {
	if t.TerminalPushSeconds == 0 { t.TerminalPushSeconds = DefaultTerminalPushSeconds }
	if t.TerminalPayWindowSeconds == 0 { t.TerminalPayWindowSeconds = DefaultTerminalPayWindowSeconds }
	if t.OnlineStartSeconds == 0 { t.OnlineStartSeconds = DefaultOnlineStartSeconds }
	if t.TerminalPushSeconds < 0 || t.TerminalPayWindowSeconds < 0 || t.OnlineStartSeconds < 0 ||
		t.TerminalPushSeconds > TerminalPushCapSeconds || t.OnlineStartSeconds > OnlineStartCapSeconds {
		return Timeouts{}, ErrTimeoutOutOfRange
	}
	return t, nil
}
```

`ErrTimeoutOutOfRange` = "a wait is negative or longer than the module allows, so the module did not start; use a value between 1 and the limit documented in Timeouts" (typed constant, house pattern). `Deps` gains `Timeouts Timeouts` (optional); `New` stores `m.timeouts`. The ports gain the wait so adapters apply it as their own client timeout: `TerminalPush.WaitSeconds int64` and `GatewayStart.WaitSeconds int64` (filled from `m.timeouts`). New typed errors: `ErrTerminalUnreachable` = "the card machine did not answer, so nothing was charged by this module; charge it on the machine and press «pago realizado correctamente»", `ErrPaymentNotPending` = "this payment is no longer pending, so it cannot be confirmed", `ErrStillPending` = "the machine has not finished yet; asking again shortly".
`Module` gains `store *orm.Store`, built once in `New`: `m.store = orm.TenantStore(db, deps.TenantID, deps.IDs)`.
Delete the hand-written persistence (create/update/upsert/patch/delete/list/get bodies that only move rows) once its rules moved to the rule files: the row-moving bodies of `CreateCounterPayment`, `StartOnlinePayment`, `StartTerminalPayment`, `RefreshTerminalPayment`, `RefundPayment`, `ReconcilePayments`.
