← [Index](PLAN.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 6 — tests (`tests/`, through `crud.LoopbackActingAsWithQueue(nil, "u1", &callQueue{}, mod)`; rows in `docs/TESTS.md`)

Delete tests of removed code.

`tests/queue_double_test.go` (copy it as is; it is the only queue the tests use):

```go
package tests

import (
	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// callQueue is the test double of crud.CallQueue: it records what crud asks and stores nothing.
type callQueue struct {
	added  []crud.CallRequest
	done   []string
	failed []string // the reasons
}

func (q *callQueue) Add(tx crud.Tx, r crud.CallRequest) (string, error) {
	q.added = append(q.added, r)
	return fmt.Sprintf("call-%d", len(q.added)), nil
}
func (q *callQueue) MarkDone(id string) error { q.done = append(q.done, id); return nil }
func (q *callQueue) MarkFailedAttempt(id string, call model.ExternalCallDefinition, reason string) error {
	q.failed = append(q.failed, reason)
	return nil
}
func (q *callQueue) ConfirmCall(id string) error        { return nil }
func (q *callQueue) RejectCall(id, reason string) error { return nil }
```

The caller of every test that touches a call is `crud.LoopbackActingAsWithQueue(nil, "u1", &callQueue{}, mod)` (keep the pointer in a variable to assert `added`, `done`, `failed`).

Operations are `router.Operation{Module: ModelName, Entity: "<entity>", Action: model.<Action>}`, never a string; fixtures follow `crud` v0.0.14 (`reg.AddStoredEntity`, `crud.CurrentUser`). Add `TestMount_MatchesDefinition` (`crud.Mount(reg, nil, nil, mod)` returns nil) and one test that an allowed write publishes a `*crud.Change` on `crud.Topic(ModelName, "<entity>", model.<Action>)`. Required:

| Test | Setup → call → expected |
|---|---|
| `TestCounter_AmountFromCharges` | counter payment with an amount from the client → `ServerOnly` error; without → amount = due |
| `TestTerminal_PushViaExternalCall` | fake terminal records pushes; create terminal payment → one push with `WaitSeconds` 5, the row reads `provider_token` from the fake, `queue.added` has `TerminalPush` and `TerminalStatus` |
| `TestTerminal_Unreachable` | fake `Push` fails → the row stays `pending` with empty `provider_token`, `queue.failed` has a reason, status is never `failed` |
| `TestTerminal_StatusPolling` | row with token; fake `Status` pending → `row.TerminalStatus(crud.CurrentUser{}, mod, "k")` returns `ErrStillPending`, row pending; fake authorized with the right amount → returns nil, row `paid` |
| `TestTerminal_WindowOverEndsPolling` | row older than the window (a `Timeouts{TerminalPayWindowSeconds: 1}` and a `CreatedAt` 10 s old) → `TerminalStatus` returns nil and the row stays `pending` |
| `TestTerminal_NoTerminalRefused` | module built without `Terminal` → create terminal payment → `ErrTerminalUnavailable` |
| `TestManualConfirmation_ClosesAsPaid` | pending terminal payment; create confirmation with a note → payment `paid`, the money facts published once; empty note → field error on `note`; confirming a paid payment → `ErrPaymentNotPending` |
| `TestOnline_ResponseCarriesRedirectUrl` | create `payment_online_request` through the loopback → the decoded response has `redirect_url` and `token` from the fake gateway; the payment row has the provider token and is `pending` |
| `TestOnline_StartFailureClosesPayment` | fake gateway `Start` fails → the payment reads `failed`, `queue.failed` has a reason |
| `TestTimeouts_DefaultsAndCap` | `New` with zero `Timeouts` → 5 / 30 / 10; `TerminalPushSeconds: 11` → `ErrTimeoutOutOfRange` |
| `TestRefund_NotMoreThanPaid` | refund above the paid amount → field error, gateway not called |
| `TestRefund_GatewayFailureKeepsPending` | fake gateway fails → `queue.failed` has a reason, the refund reads `pending` |
| `TestOnline_ForgedCancelIgnored` | pending online payment; fake gateway `Confirm` reports pending; `CompleteOnlinePayment(OnlineReturn{PaymentId: id})` (what a forged `TBK_ORDEN_COMPRA` produces) → payment still pending |
| `TestOnline_ConfirmedByGateway` | fake `Confirm` reports authorized with the right amount → payment paid; reports cancelled → payment cancelled |
| `TestOnline_TokenMismatchRejected` | `OnlineReturn{PaymentId: id, Token: "other"}` → `ErrTokenMismatch`, payment unchanged |
| `TestDemo_Valid` (`//go:build wasm`) | `seed.Demo()` → no error, `Validate()` nil, at least one screen |
