---
PLAN: "feat: card terminal port, balance per reference, provider per payment"
TAG: v0.2.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 4152317682437491582
---

> Read AGENTS.md at the repo root before any change.

# Plan — card terminal, balance per reference, provider per payment

## Why

A clinic charges at a till with a card machine (TUU today, another brand tomorrow). Today the cashier
types the amount into the machine by hand. After this plan the system **pushes the amount to the
machine** through a new port `Terminal`, waits for the result and records the payment. The module
also stops refusing a second payment on a reference that still owes money (**balance**), and every
payment records **which processor** handled it, so a later change of processor never loses track of
old payments.

Language: identifiers, keys and log/error strings in English; **comments and docs in Spanish**
(this repo is `veltylabs/*`).

## Delivery rules

- Never pause to ask permission or to report progress. When the Delivery check passes: commit, push,
  open the PR. Stuck for 30 minutes on one point: open the PR anyway, listing what fails.
- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never edit `model_orm.go` by hand: regenerate it with `ormc`. A build error there means the
  hand-written `model.go` is wrong.
- Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`; run `go mod tidy`
  before `ormc`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/ormc/cmd/ormc@latest
go install webtyp.com/lang/cmd/langc@latest
```

## Ecosystem search

Searched `webtyp/*` and `veltylabs/*` for `Terminal`, `POS`, `device`, `card machine`, `balance`,
`due`. Nothing models a card machine: `payment_manager.Gateway` is the only payment port and it is
redirect-shaped (browser goes to a payment page). **New** port `Terminal` next to `Gateway` in this
repo (same owner of the payment state). Balance: **extend** this repo (`referencePaid`/`chargeFor` in
`module.go`). The concrete TUU adapter is a separate repo (`github.com/veltylabs/tuu`, later plan),
exactly like `github.com/veltylabs/transbank` for `Gateway`.

## Design gate

1. **Prior art.** Stripe Terminal (`PaymentIntent` pushed to a reader, client polls/gets webhook),
   Adyen Terminal API (`PaymentRequest` to a `POIID`, async result), Mercado Pago Point (order sent to a
   device id, poll status). All three: the server sends *amount + device id + idempotency key*, the
   device answers *pending → approved/rejected/cancelled*. `Terminal.Push` + `Terminal.Status` is that
   shape; we poll because TUU's remote payment documents no callback.
2. **Novice names.** `Terminal` (the industry word: "payment terminal"), `Push` ("push the amount to the
   terminal"), `Status`, `Provider()` ("which processor is this"), `StartTerminalPayment`,
   `RefreshTerminalPayment`, `ReconcilePayments` (no longer only online, so the old name
   `ReconcileOnlinePayments` is renamed now), `Due` ("what is still due").
3. **Complexity ledger.** Concepts +2 (`Terminal`, balance) / −0 · Files to touch to add a processor:
   1 adapter repo, 0 lines here · Call site in an app: +1 field (`Deps.Terminal`) +1 field per
   `ChargeSource` (`Device`) · Ways to charge a card at the till: **1** — when the reference module has a
   device, `CreateCounterPayment` with debit/credit is refused (`ErrTerminalRequired`); without a device,
   manual card recording stays the only way.
4. **Where it belongs.** The port lives where the payment state lives (this repo); implementations live
   in adapter repos; `sandbox/` gets the in-memory one, as it already has for `Gateway`.
5. **What it deletes.** `ReconcileOnlinePayments`, `OpReconcileOnlinePayments`,
   `ReconcileOnlinePaymentsArgs(Model)`, `oldPendingOnlinePayments`, `referencePaid`, `chargeFor`,
   `OnlinePendingTimeout` (replaced by `PendingTimeout`). `Gateway` gains `Provider()` — a breaking change
   for `github.com/veltylabs/transbank`, adapted in its own plan after this tag.

## APIs used

| Symbol | Signature | Tag | File |
|---|---|---|---|
| `time.AfterFunc` | `func AfterFunc(milliseconds int, f func()) Timer` | `webtyp.com/time v0.6.2` | `api.go` |
| `time.Timer` | `interface{ Stop() bool }` | `webtyp.com/time v0.6.2` | `api.go` |
| `time.NowUnix` | `func NowUnix() Unix` | `webtyp.com/time v0.6.2` | `api.go` |

Everything else used is already used in this repo (`orm`, `model`, `router`, `money`, `events`, `dom`).
If a signature in the published tag differs from this table, stop and name the symbol in the PR;
never adapt it by guessing.

## Stage 1 — Balance per reference (`module.go`, `ops.go`)

Replace `referencePaid` and `chargeFor` (delete both) with one exported method:

```go
// Due devuelve lo que falta cobrar de una referencia: el monto que informa su ChargeReader menos el
// bruto de los pagos PAID y PARTIALLY_REFUNDED de esa referencia. Un pago PENDING, FAILED, CANCELLED,
// EXPIRED o REFUNDED no cuenta. El monto nunca viene del cliente.
func (m *Module) Due(tenantId, referenceModule, referenceId string) (amount int64, currency string, err error) {
	payments, err := m.repo.paymentsByReference(tenantId, referenceModule, referenceId)
	if err != nil {
		return 0, "", err
	}
	var covered int64
	for _, p := range payments {
		if p.Status == StatusPaid || p.Status == StatusPartiallyRefunded {
			covered += p.Amount
		}
	}
	total, currency, payable, err := m.Charge(tenantId, referenceModule, referenceId)
	if err != nil {
		return 0, "", err
	}
	if !payable {
		if covered > 0 {
			return 0, "", ErrAlreadyPaid
		}
		return 0, "", ErrNotPayable
	}
	if total-covered <= 0 {
		return 0, "", ErrAlreadyPaid
	}
	return total - covered, currency, nil
}
```

`CreateCounterPayment` and `StartOnlinePayment` call `m.Due(...)` where they called `m.chargeFor(...)`;
the payment amount is the returned `amount`.

`opGetCharge` answers with `Due` instead of `Charge`:

```go
amount, currency, err := m.Due(a.TenantId, a.ReferenceModule, a.ReferenceId)
switch err {
case nil:
	_ = ctx.Encode(&ChargeResult{Amount: amount, Currency: currency, Payable: true, Terminal: m.hasTerminal(a.ReferenceModule)})
case ErrAlreadyPaid, ErrNotPayable:
	_ = ctx.Encode(&ChargeResult{Payable: false})
default:
	writeError(ctx, err)
}
```

(`Terminal` and `hasTerminal` come in Stage 2; in Stage 1 write `Payable: true` only and add the field in
Stage 2.)

| Charge (amount, payable) | Payments of the reference | `Due` |
|---|---|---|
| 30000, true | none | 30000 |
| 30000, true | PAID 10000 | 20000 |
| 30000, true | PAID 30000 | `ErrAlreadyPaid` |
| 30000, true | PARTIALLY_REFUNDED 30000 (5000 refunded) | `ErrAlreadyPaid` |
| 30000, true | REFUNDED 30000 | 30000 |
| 30000, true | PENDING 30000 (online) | 30000 |
| 30000, false | PAID 30000 | `ErrAlreadyPaid` |
| 0, false | none | `ErrNotPayable` |

Tests (new file `tests/due_test.go`, one test per row above): setup `newEnv(t)`, set `e.charge.amount`
/ `e.charge.payable`, create the listed payments with the existing helpers (`paidCounter`, `started`,
`RefundPayment`); for "PAID 10000" create a counter payment while `e.charge.amount = 10000`, then set it
to 30000 → call `e.m.Due(tenant, refModule, refID)` → expect the column. Plus one test: after "PAID
10000", `CreateCounterPayment` with a new id creates a payment of **20000**.

Existing tests that expect `ErrAlreadyPaid` keep passing unchanged (same amount on both payments).

## Stage 2 — The `Terminal` port (`ports.go`, `model.go`, `errors.go`, `module.go`)

### `ports.go` — add

```go
// Terminal es la máquina de cobro con tarjeta de una caja. La implementación concreta (por ejemplo
// github.com/veltylabs/tuu) la inyecta la app; sus llamadas son síncronas y solo corren en el servidor.
type Terminal interface {
	Provider() string                               // nombre del procesador; se guarda en cada pago ("tuu")
	Push(req TerminalPush) (TerminalPushed, error)  // envía el monto a la máquina; idempotente por RequestId
	Status(token string) (TerminalResult, error)    // estado del cobro en la máquina
}

type TerminalPush struct {
	RequestId   string // = Payment.Id; la máquina lo usa como llave de idempotencia
	Device      string // id de la máquina en el procesador (TUU: número de serie)
	Amount      int64
	Currency    string
	Method      string // MethodDebit | MethodCredit
	Description string // texto corto que muestra la máquina; puede ir vacío
}

type TerminalPushed struct {
	Token string // id del cobro en el procesador; se guarda en Payment.ProviderToken
}

type TerminalResult struct {
	Outcome           string // OutcomePending | OutcomeAuthorized | OutcomeRejected | OutcomeCancelled
	Amount            int64
	AuthorizationCode string
	CardLast4         string
}
```

Add to the `Outcome*` constants: `OutcomeCancelled = "cancelled"`.

Add to the `Gateway` interface, first method: `Provider() string // nombre del procesador ("transbank")`.

`ChargeSource` gains one field:

```go
type ChargeSource struct {
	Module string
	Reader ChargeReader
	Device string // id de la máquina de la caja que cobra las referencias de este módulo; "" = sin máquina
}
```

### `model.go`

- `ChannelTerminal = "terminal"` in the `Channel*` block (comment: `// cobro en caja empujado a la máquina de tarjeta`).
- Rename `OnlinePendingTimeout` → `PendingTimeout` (same value 600, comment: `// un pago PENDING (online o máquina) más viejo que esto se concilia`).
- `PaymentModel`: new field after `provider_token`:
  `{Name: "provider", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 30}},`
- `ChargeResultModel`: new field `{Name: "terminal", Type: model.Bool()},`
- New args model:

```go
var StartTerminalPaymentArgsModel = model.Definition{
	Name: "start_terminal_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		{Name: "method", Type: input.Radio(), NotNull: true}, // MethodDebit | MethodCredit
	},
}
```

- Rename `ReconcileOnlinePaymentsArgsModel` → `ReconcilePaymentsArgsModel`, `Name: "reconcile_payments_args"`.

Run `go mod tidy && ormc` to regenerate `model_orm.go` (it then has `StartTerminalPaymentArgs`,
`ReconcilePaymentsArgs`, `Payment.Provider`, `ChargeResult.Terminal`).

### `errors.go` — add, verbatim

```go
ErrTerminalUnavailable = domainError("card terminal payments are not configured")
ErrNoTerminalDevice    = domainError("no card terminal device for this reference module")
ErrTerminalFailed      = domainError("card terminal call failed")
ErrTerminalRequired    = domainError("card payments for this reference go through the card terminal")
ErrNotTerminal         = domainError("payment is not a card terminal payment")
```

`StatusFor`: `ErrNoTerminalDevice`, `ErrTerminalRequired`, `ErrNotTerminal` → 400; `ErrTerminalUnavailable`,
`ErrTerminalFailed` → 500 (default).

### `module.go`

- `Deps` gains `Terminal Terminal // opcional — nil: el cobro con máquina responde ErrTerminalUnavailable`.
- `Module` gains `term Terminal`.
- `New`: if any `ChargeSource.Device != ""` and `deps.Terminal == nil` →
  `fmt.Err("payment_manager: Deps.Terminal is required when a ChargeSource names a Device")`.
- `source(referenceModule) (ChargeSource, bool)` private helper; `Charge` uses it.
- `hasTerminal(referenceModule string) bool` = `m.term != nil` and the source has a non-empty `Device`.
- `StartOnlinePayment`: sets `p.Provider = m.gw.Provider()`.
- `CreateCounterPayment`: after `isCounterMethod`, if `(cmd.Method == MethodDebit || cmd.Method == MethodCredit) && m.hasTerminal(cmd.ReferenceModule)` → `ErrTerminalRequired`.

New method:

```go
// StartTerminalPayment crea el pago PENDING y envía el monto a la máquina de la caja del módulo de la
// referencia. Repetir el mismo id con la misma referencia devuelve el pago guardado.
func (m *Module) StartTerminalPayment(cmd StartTerminalPaymentArgs) (Payment, error)
```

Order of checks (each one a test):
1. `m.term == nil` → `ErrTerminalUnavailable`.
2. any of `TenantId, Id, ReferenceModule, ReferenceId, Method` empty → `ErrMissingArgs`.
3. `m.repo.paymentAnyTenant(cmd.Id)` found: same tenant + module + reference → return it, nil; else → `ErrIdTaken`.
4. method not `MethodDebit`/`MethodCredit` → `ErrInvalidMethod`.
5. source without `Device` → `ErrNoTerminalDevice`.
6. `amount, currency, err := m.Due(...)`; err → return it.
7. Build `Payment{Channel: ChannelTerminal, Method: cmd.Method, Amount: amount, Currency: currency, Status: StatusPending, Provider: m.term.Provider(), CreatedAt/UpdatedAt: now, Revision: 1, ...}`.
8. `pushed, err := m.term.Push(TerminalPush{RequestId: p.Id, Device: src.Device, Amount: amount, Currency: currency, Method: cmd.Method})`;
   error → `p.Status = StatusFailed`, `m.create(&p)`, return `ErrTerminalFailed`.
9. `p.ProviderToken = pushed.Token`; `m.create(&p)`; return `p`.

A terminal payment never gets a `ReceiptNumber`: the machine's voucher is the tax document.

```go
// RefreshTerminalPayment consulta la máquina y asienta el resultado de un pago PENDING. Un pago que ya
// no está PENDING se devuelve tal cual.
func (m *Module) RefreshTerminalPayment(tenantId, id string) (Payment, error)
```

1. `p, err := m.repo.paymentById(tenantId, id)`; err → return it.
2. `p.Channel != ChannelTerminal` → `ErrNotTerminal`.
3. `p.Status != StatusPending` → return `p, nil`.
4. `m.term == nil` → `ErrTerminalUnavailable`.
5. `res, err := m.term.Status(p.ProviderToken)`; err → `p, ErrTerminalFailed`.
6. `return m.settleTerminal(p, res, false)`.

```go
// settleTerminal asienta lo que informa la máquina. fromReconcile: un resultado aún pendiente vence.
func (m *Module) settleTerminal(p Payment, res TerminalResult, fromReconcile bool) (Payment, error) {
	switch res.Outcome {
	case OutcomeAuthorized:
		if res.Amount != p.Amount {
			failed, err := m.close(p, EventFail)
			if err != nil {
				return failed, err
			}
			return failed, ErrAmountMismatch
		}
		p.AuthorizationCode = res.AuthorizationCode
		p.CardLast4 = res.CardLast4
		p.PaidAt = time.NowUnix()
		return m.close(p, EventPay)
	case OutcomeRejected:
		return m.close(p, EventFail)
	case OutcomeCancelled:
		return m.close(p, EventCancel)
	}
	if fromReconcile {
		return m.close(p, EventExpire)
	}
	return p, nil
}
```

| Case | Machine says | Result |
|---|---|---|
| T1 | push ok, then `authorized` 25000 | PAID, `payment.paid` + money fact published, provider "sandbox" |
| T2 | `rejected` | FAILED, `payment.failed` |
| T3 | `cancelled` | CANCELLED |
| T4 | `pending` | stays PENDING, revision unchanged |
| T5 | `authorized` 24000 for a 25000 payment | FAILED + `ErrAmountMismatch` |
| T6 | push error | payment saved FAILED, `ErrTerminalFailed` |
| T7 | status error | `ErrTerminalFailed`, payment stays PENDING |
| T8 | counter debit on a module with device | `ErrTerminalRequired` |
| T9 | same id twice | second call returns the first payment, one `Push` only |
| T10 | refresh of a counter payment | `ErrNotTerminal` |

### Rename reconcile, cover terminal payments

- `ReconcileOnlinePayments` → `ReconcilePayments(tenantId string, now time.Unix) (int, error)`.
- `repository.oldPendingOnlinePayments` → `oldPendingPayments(tenantId string, channel string, before int64)`
  (same query, channel as parameter).
- `ReconcilePayments` = the current online loop (only when `m.gw != nil`, channel `ChannelOnline`) **plus** the
  same loop for `ChannelTerminal` when `m.term != nil`: older than `GatewayStatusWindow` → `close(p, EventExpire)`;
  otherwise `m.term.Status(p.ProviderToken)` (error → `continue`) and `settleTerminal(p, res, true)`; the
  same `switch err` on the result as the online loop. Returns the total closed.

## Stage 3 — Operations (`ops.go`)

- `OpStartTerminalPayment = "start_terminal_payment"` → `Requires(ResourcePayment, model.Create).Accepts(&StartTerminalPaymentArgs{})`, answers `public(p)`.
- `OpRefreshTerminalPayment = "refresh_terminal_payment"` → `Requires(ResourcePayment, model.Update).Accepts(&GetPaymentArgs{})`, answers `public(p)`.
- `OpReconcileOnlinePayments` → `OpReconcilePayments = "reconcile_payments"`, handler `opReconcilePayments`, args `ReconcilePaymentsArgs`.
- `opGetCharge` as in Stage 1, with `Terminal: m.hasTerminal(a.ReferenceModule)`.

Tests: extend `tests/ops_test.go` tables (lines 68, 167, 204 list the ops): add the two new ops with their
resource/action, replace `OpReconcileOnlinePayments` by `OpReconcilePayments`; `tests/models_test.go`
lines 48 and 80: `reconcile_online_payments_args` → `reconcile_payments_args`, `ReconcilePaymentsArgs(List)`.

`ui/paymentsview.go` lines 111–112 call `OpReconcilePayments` with `ReconcilePaymentsArgs`. Its button text
key changes from `"Reconcile online payments"` to `"Reconcile payments"` (Spanish `"Conciliar pagos"`);
`tests/ui_browser_test.go` lines 218 and 231 click the new text.

## Stage 4 — `sandbox.Terminal` (`sandbox/terminal.go`, new)

Copy the shape of `sandbox.Gateway` (`sandbox/sandbox.go`): slice of transactions, never a map.

```go
// Terminal es una máquina de tarjeta en memoria que implementa paymentmanager.Terminal.
type Terminal struct { /* txs []terminalTx; failNext error */ }

const TerminalProvider = "sandbox"
const TerminalTokenPrefix = "sbt-" // antecede al RequestId en cada token

var _ paymentmanager.Terminal = (*Terminal)(nil)

func NewTerminal() *Terminal
func (t *Terminal) Provider() string                 // TerminalProvider
func (t *Terminal) Push(req paymentmanager.TerminalPush) (paymentmanager.TerminalPushed, error) // same RequestId → same token, no new tx
func (t *Terminal) Status(token string) (paymentmanager.TerminalResult, error) // unknown token → ErrUnknownToken
func (t *Terminal) Approve(token string)             // Outcome authorized, Amount = pushed amount, AuthorizationCode = AuthorizationCode, CardLast4 = "4242"
func (t *Terminal) Reject(token string)
func (t *Terminal) Cancel(token string)
func (t *Terminal) SetAmount(token string, amount int64)
func (t *Terminal) FailNext(err error)
func (t *Terminal) Pushes() int                       // how many distinct Push calls created a tx (for test T9)
```

`sandbox.Gateway` gains `func (g *Gateway) Provider() string { return "sandbox" }`.

Tests: `tests/terminal_test.go` with cases T1–T10 above. In `tests/setup_test.go`, `build` gets a
`withTerminal bool`: when true, `e.term = sandbox.NewTerminal()`, `deps.Terminal = e.term`, and the
`ChargeSource` gets `Device: "POS-1"`. Keep `newEnv` (gateway, no terminal) and add
`newEnvTerminal(t)` (gateway + terminal). `hookGateway` gets `Provider()` from the embedded sandbox.
`tests/reconcile_test.go` switches to `ReconcilePayments` and `pm.PendingTimeout`, plus two cases: a
terminal payment PENDING past `PendingTimeout` that the machine reports authorized → closed PAID; one past
`GatewayStatusWindow` → EXPIRED.

## Stage 5 — Screen step (`ui/paymentstep.go`, `lang.json`)

When `ChargeResult.Terminal` is true and the chosen method is debit or credit, the step pushes to the
machine instead of registering a counter payment:

```go
func (s *PaymentStep) registerCounter() {
	if s.terminal && (s.method == paymentmanager.MethodDebit || s.method == paymentmanager.MethodCredit) {
		s.startTerminal()
		return
	}
	// … current body unchanged …
}

func (s *PaymentStep) startTerminal() {
	out := &paymentmanager.Payment{}
	args := &paymentmanager.StartTerminalPaymentArgs{TenantId: s.TenantID, Id: s.IDs.NewID(), ReferenceModule: s.ReferenceModule, ReferenceId: s.ReferenceID, Method: s.method}
	s.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpStartTerminalPayment, args, out, func(err error) {
		if err != nil {
			s.busy.Set(false)
			s.errMsg.Set(err.Error())
			return
		}
		s.amount.Set(lang.Translate(textWaitingCard).String())
		s.pollTerminal(out.Id)
	})
}

// pollTerminal consulta la máquina cada terminalPollMillis hasta que el pago deja de estar PENDING.
func (s *PaymentStep) pollTerminal(id string) {
	time.AfterFunc(terminalPollMillis, func() {
		out := &paymentmanager.Payment{}
		s.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpRefreshTerminalPayment, &paymentmanager.GetPaymentArgs{TenantId: s.TenantID, Id: id}, out, func(err error) {
			switch {
			case err != nil:
				s.busy.Set(false)
				s.errMsg.Set(err.Error())
			case out.Status == paymentmanager.StatusPending:
				s.pollTerminal(id)
			case out.Status == paymentmanager.StatusPaid:
				s.busy.Set(false)
				if s.OnPaid != nil {
					s.OnPaid(out.Id)
				}
			default: // FAILED, CANCELLED, EXPIRED
				s.busy.Set(false)
				s.errMsg.Set(lang.Translate(lang.Text(out.Status)).String())
			}
		})
	})
}
```

- New field `terminal bool` set in `loadCharge` from `out.Terminal`.
- `const terminalPollMillis = 3000` and `textWaitingCard lang.Text = "Waiting for the card on the machine…"`.
- Import `webtyp.com/time` (it is already in `go.mod`).
- `lang.json`: run `langc sync .`; the new key must read
  `"Waiting for the card on the machine…": ["Esperando la tarjeta en la máquina…"],`
  (status keys `FAILED`, `CANCELLED`, `EXPIRED` already exist).

Test in `tests/ui_browser_test.go` (copy `TestWASM_PaymentStep_CounterRegistersAndCallsOnPaid`, line 84):
with `newEnvTerminal`, click "Debit" then "Register payment" → one payment with `Channel == ChannelTerminal`
and `Status == StatusPending` exists; `e.term.Approve(<its ProviderToken>)` right away; then wait in a loop
(`time.Sleep(200ms)`, at most 5 s) until `OnPaid` was called → expect it called once with that payment id.
The poll interval stays a private constant (never exported for a test).

## Stage 6 — Docs (Spanish)

- `README.md`: operations table — add `start_terminal_payment`, `refresh_terminal_payment`, rename
  `reconcile_online_payments` → `reconcile_payments` ("Cierra pagos online y de máquina pendientes"); state
  `CANCELLED` also covers "cancelado en la máquina"; wiring example adds `Terminal: pos` and
  `Device: "<serie de la máquina>"` on the `ChargeSource`; line "La app corre `reconcile_payments` cada minuto".
- `docs/ARCHITECTURE.md` § Puertos: add `Terminal` (one paragraph: what it is, polling, voucher = boleta, no
  receipt number); § Reglas de dinero: the balance rule of `Due` with the table of Stage 1.
- `docs/diagrams/flows.md`: §3 replace rhombus C5 by `C5: ¿Due > 0?`; new section `## 11. Cobro con máquina
  (StartTerminalPayment / RefreshTerminalPayment)` as a `flowchart TD` with ids `T1`…`T10` matching the
  table above; §8 renamed `Conciliar (ReconcilePayments)` with the terminal branch. Remove the sentence that
  links `docs/PLAN.md` at the top of `flows.md` (permanent docs never link the plan): replace it with
  "Cada rama tiene una prueba en `tests/` con su id en el nombre o en un comentario."

## Deletions — each must grep empty

```bash
grep -rn "ReconcileOnlinePayments\|reconcile_online_payments\|OnlinePendingTimeout\|oldPendingOnlinePayments\|referencePaid\|chargeFor" --include=*.go --include=*.md --include=*.json .
grep -n "PLAN.md" docs/diagrams/flows.md README.md docs/ARCHITECTURE.md
```

## Delivery check

```bash
go mod tidy && ormc && langc sync . && gotest
```

All green, the two greps above empty, `git diff --stat main` lists every file named in the stages.

## Stages

| # | What | Files |
|---|---|---|
| 1 | Balance `Due` | `module.go`, `ops.go`, `tests/due_test.go` |
| 2 | `Terminal` port, model, errors, start/refresh/settle, reconcile rename | `ports.go`, `model.go`, `model_orm.go` (generated), `errors.go`, `module.go`, `repository.go` |
| 3 | Operations | `ops.go`, `tests/ops_test.go`, `tests/models_test.go` |
| 4 | Sandbox terminal + tests | `sandbox/terminal.go`, `sandbox/sandbox.go`, `tests/setup_test.go`, `tests/terminal_test.go`, `tests/reconcile_test.go`, `tests/tenant_test.go` |
| 5 | Payment step pushes to the machine | `ui/paymentstep.go`, `ui/paymentsview.go`, `lang.json`, `tests/ui_browser_test.go` |
| 6 | Docs | `README.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/flows.md` |
