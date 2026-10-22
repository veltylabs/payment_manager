---
PLAN: "feat!: card payouts — deposit date and processor fee per payment; facts carry means and actor"
TAG: v0.4.0
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.

<!-- Parked. Dispatch after this repo's PLAN_GATEWAY.md (v0.3.0) is closed AND webtyp.com/money v0.2.0 is published.
The dispatcher renames this file to docs/PLAN.md and writes the real tags in "Pinned versions". -->

# Plan — when and how much a card payment really deposits

## Why

With a card the money does not arrive the same day nor whole: the processor deposits it N **bank business days**
later, minus its fee and the IVA of that fee. Each processor and each contracted plan computes it differently (TUU:
1.49 % flat or 0.79 % + $65 per sale, deposit in 1/2/5/10 days; Transbank: another model). So:

1. A new optional capability `PayoutTerms` that an adapter (`Terminal` or `Gateway`) implements with its own plan.
2. Every money fact says **how** it was paid (`Means`) and **who** registered it (`Actor`) — `webtyp.com/money` v0.2.0.
3. Cash and transfer publish `Charged` (the money is in hand). A card publishes an `Accrued` **inflow** (owed by the
   processor, `DueAt` = deposit day) and an `Accrued` **outflow** (fee + its IVA). The cash shift then counts only
   cash, and the 30-day cash view shows card money on the day it lands.

Example: $30.000 by card on Friday 2026-10-09, TUU flat plan with 2-day payout → `Accrued` inflow 30000 due Tuesday
2026-10-13, `Accrued` outflow 532 (fee 447 + IVA 85) due the same day.

Language: identifiers, keys and error strings in English; **README, docs, diagrams and comments in Spanish**, each
exported symbol with a `// Ejemplo: …` line.

## Delivery rules

- **Commit after every stage.** Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash`.
- Never pause to ask permission or to report progress. When the Delivery check passes: push and open the PR. Stuck
  for 30 minutes: open the PR anyway, listing what fails.
- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never edit `model_orm.go` by hand: regenerate it with `ormc`.
- The code blocks below are the implementation: copy them.

## Tools and pinned versions

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/ormc/cmd/ormc@latest
export GOPRIVATE='webtyp.com/*,github.com/veltylabs/*' GOFLAGS=-mod=mod
go get webtyp.com/money@v0.2.0 && go mod tidy   # money v0.2.0 publicado y verificado 2026-10-22
```

## APIs used (`webtyp.com/money` v0.2.0)

`money.Fact` gains `Means money.Means` and `Actor string`; `money.MeansCash`, `money.MeansCard`, `money.MeansTransfer`;
`Validate` rejects `Charged`/`Refunded` without a valid `Means` (`money.ErrMeans`). `money.Share(m Money, bp int64)
(Money, error)`. If the published tag differs, stop and name it in the PR.

## Stage 1 — Ports (`ports.go`)

```go
// PayoutTerms es lo que un adaptador (Terminal o Gateway) sabe de su plan contratado: cuánto descuenta el
// procesador y en cuántos días hábiles bancarios deposita. Es opcional: el módulo lo detecta con una aserción
// de tipo; sin él, un cobro con tarjeta se registra sin fecha de depósito ni comisión.
// Ejemplo: terms, ok := gateway.(paymentmanager.PayoutTerms)
type PayoutTerms interface {
	Payout(c PayoutCharge) (Payout, error)
}

// PayoutCharge es el cobro del que se pregunta el abono.
type PayoutCharge struct {
	Amount   int64
	Currency string
	Method   string // MethodDebit, MethodCredit o MethodCard
	Channel  string // ChannelTerminal o ChannelOnline
}

// Payout es lo que responde el adaptador: comisión, IVA de la comisión y días hábiles hasta el depósito.
// El neto depositado es Amount − Fee − FeeTax.
type Payout struct {
	Fee          int64
	FeeTax       int64
	BusinessDays int
}

// BankDays convierte días hábiles bancarios en fecha. Lo cumple business_calendar.(*Module).BankDayAfter.
// Ejemplo: deps.BankDays = calendario
type BankDays interface {
	BankDayAfter(from time.Unix, days int) (time.Unix, error)
}
```

(`ports.go` imports `webtyp.com/time`.) **Commit.**

## Stage 2 — Wiring and who registered it (`module.go`, `model.go`, `ops.go`)

- `Deps` gains `BankDays BankDays // requerido si el Terminal o el Gateway implementan PayoutTerms`. `New`: if
  `deps.Terminal` or `deps.Gateway` implements `PayoutTerms` and `deps.BankDays == nil` →
  `fmt.Err("payment_manager: Deps.BankDays is required when an adapter implements PayoutTerms")`. `Module` keeps
  `bank BankDays`.
- `CreateCounterPaymentArgsModel`, `StartTerminalPaymentArgsModel`, `StartOnlinePaymentArgsModel` and
  `RefundPaymentArgsModel` gain `{Name: "created_by", Type: model.Text()}`; `ormc`.
- The four module methods copy `cmd.CreatedBy` into the new `Payment.CreatedBy` / `Refund.CreatedBy`.
- In `ops.go`, each of the four handlers sets `a.CreatedBy = ctx.UserID()` right after `decode` (never trust the body).

**Commit.**

## Stage 3 — Facts (`facts.go`, rewritten complete)

```go
package paymentmanager

import "webtyp.com/money"

// Origen de los hechos.
const (
	SourcePayment    = "payment"
	SourcePaymentFee = "payment_fee"
	SourceRefund     = "refund"
)

// meansOf traduce el medio del pago al medio del hecho.
func meansOf(method string) money.Means {
	switch method {
	case MethodCash:
		return money.MeansCash
	case MethodTransfer:
		return money.MeansTransfer
	}
	return money.MeansCard // debit, credit, card
}

// paymentFacts arma los hechos de un pago recién pagado: Charged si el dinero está en mano; con tarjeta, lo que
// debe el procesador (Accrued de entrada, vence el día del depósito) y su comisión (Accrued de salida).
func (m *Module) paymentFacts(p Payment) ([]money.Fact, error) {
	amt, err := money.New(p.Amount, money.Currency(p.Currency))
	if err != nil {
		return nil, err
	}
	means := meansOf(p.Method)
	base := money.Fact{Amount: amt, Scope: p.ReferenceModule, Source: money.Ref{Type: SourcePayment, ID: p.Id}, At: p.PaidAt, Means: means, Actor: p.CreatedBy}
	if means != money.MeansCard {
		base.Kind, base.Side, base.Party = money.Charged, money.Inflow, p.ReferenceId
		return []money.Fact{base}, nil
	}
	base.Kind, base.Side, base.Party = money.Accrued, money.Inflow, p.Provider
	terms, ok := m.payoutTerms(p.Channel)
	if !ok {
		return []money.Fact{base}, nil
	}
	po, err := terms.Payout(PayoutCharge{Amount: p.Amount, Currency: p.Currency, Method: p.Method, Channel: p.Channel})
	if err != nil {
		return nil, err
	}
	due, err := m.bank.BankDayAfter(p.PaidAt, po.BusinessDays)
	if err != nil {
		return nil, err
	}
	base.DueAt = due
	facts := []money.Fact{base}
	if cost := po.Fee + po.FeeTax; cost > 0 {
		feeAmt, err := money.New(cost, money.Currency(p.Currency))
		if err != nil {
			return nil, err
		}
		facts = append(facts, money.Fact{Kind: money.Accrued, Side: money.Outflow, Amount: feeAmt, Scope: p.ReferenceModule,
			Source: money.Ref{Type: SourcePaymentFee, ID: p.Id}, Party: p.Provider, At: p.PaidAt, DueAt: due, Means: money.MeansCard, Actor: p.CreatedBy})
	}
	return facts, nil
}

// payoutTerms devuelve el PayoutTerms del adaptador del canal, si lo implementa.
func (m *Module) payoutTerms(channel string) (PayoutTerms, bool) {
	var a any
	switch channel {
	case ChannelTerminal:
		a = m.term
	case ChannelOnline:
		a = m.gw
	}
	t, ok := a.(PayoutTerms)
	return t, ok
}

// refundFact arma el hecho de una devolución, con el medio del pago devuelto.
func refundFact(p Payment, r Refund) (money.Fact, error) {
	amt, err := money.New(r.Amount, money.Currency(p.Currency))
	if err != nil {
		return money.Fact{}, err
	}
	return money.Fact{Kind: money.Refunded, Side: money.Outflow, Amount: amt, Scope: p.ReferenceModule,
		Source: money.Ref{Type: SourceRefund, ID: r.Id}, Party: p.ReferenceId, At: r.CreatedAt, Means: meansOf(p.Method), Actor: r.CreatedBy}, nil
}

```

Every `money.Publish(m.pub, paymentFact(p))` (`module.go` lines ~191 and ~443) becomes:

```go
		facts, err := m.paymentFacts(p)
		if err != nil {
			return Payment{}, err
		}
		for _, f := range facts {
			if err := money.Publish(m.pub, f); err != nil {
				return Payment{}, err
			}
		}
```

and the refund one (~532) uses `refundFact` with its error. Delete the old `paymentFact`.

| Case | Payment | Facts |
|---|---|---|
| F1 | cash 25000, by u1 | `Charged` Inflow 25000, `Means` CASH, `Actor` u1, `Party` = reference |
| F2 | transfer | `Charged`, `Means` TRANSFER |
| F3 | terminal debit 30000, adapter without `PayoutTerms` | `Accrued` Inflow 30000, `Means` CARD, `DueAt` 0, `Party` = provider |
| F4 | terminal debit 30000 paid Fri 2026-10-09, terms `{Fee 447, FeeTax 85, BusinessDays 2}`, fake bank days returns Tue 2026-10-13 | `Accrued` Inflow 30000 due 13th + `Accrued` Outflow 532 due 13th, Source `payment_fee` |
| F5 | online card, gateway with terms | same as F4 |
| F6 | refund of F1 by u2 | `Refunded` Outflow, `Means` CASH, `Actor` u2 |
| F7 | `New` with a terminal implementing `PayoutTerms` and no `BankDays` | error |

Tests in `tests/facts_test.go` (rewrite): a fake terminal/gateway that also implements `PayoutTerms`, a fake
`BankDays` returning a fixed date. Fix every existing test that asserted the old single `Charged` for card payments.
**Commit.**

## Stage 4 — Sandbox terms (`sandbox/`)

`sandbox.Terminal` and `sandbox.Gateway` gain `SetPayout(p paymentmanager.Payout)` and implement `PayoutTerms`
only when it was set (use a separate wrapper type `sandbox.WithPayout{…}` that embeds the adapter and adds the method,
so a plain sandbox keeps not implementing it). **Commit.**

## Stage 5 — Docs (Spanish)

`README.md`: section "Abono y comisión de tarjetas" with the $30.000 example and the wiring
`Deps{Terminal: pos, BankDays: calendario, …}`. `docs/ARCHITECTURE.md`: the facts table above. `docs/diagrams/flows.md`:
new section with a `flowchart TD` of `paymentFacts`. **Commit.**

## Stage 6 — Spanish (only what is still in English)

Every `README.md`, `docs/*` file, diagram and code comment in Spanish; translate only what is still English; each
exported symbol's comment has a `// Ejemplo: …` line. **Commit.**

## Deletions — each must grep empty

```bash
grep -rn "func paymentFact(" --include=*.go .
```

## Delivery check

```bash
go mod tidy && ormc && gotest
```

## Stages

| # | What | Files |
|---|---|---|
| 1 | Ports | `ports.go` |
| 2 | Wiring, created_by | `module.go`, `model.go`, `model_orm.go` (generated), `ops.go` |
| 3 | Facts | `facts.go`, `module.go`, `tests/facts_test.go` |
| 4 | Sandbox terms | `sandbox/*` |
| 5 | Docs | `README.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/flows.md` |
| 6 | Spanish | every English `*.md` and `*.go` comment |
