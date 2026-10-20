---
PLAN: "feat(money): auto-generated receipt numbers and publish economic facts to webtyp.com/money"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 4223631125565401932
PR: https://github.com/veltylabs/payment_manager/pull/3
REVIEWED_COMMIT: 03718bf96a7b58cd08c2ee844043f1b35a560a69
---

> Read AGENTS.md at the repo root before any change. This plan is complete: do not guess; if something does not fit, stop and ask in the PR.

## Goal

Two independent, additive changes, both from `webtyp/docs/MONEY_FLOWS_MASTER_PLAN.md` phase M2 (history:
`webtyp/docs/MONEY_FLOWS_DESIGN.md`):

1. **`receipt_number` stops being caller-supplied and becomes system-generated.** Today
   `CreateCounterPayment` copies `cmd.ReceiptNumber` verbatim (`module.go:156`) — the caller invents it. The
   administradora needs it to be the system's own number, unique per tenant, sequential, never reused.
2. **Every successful charge and refund publishes a `money.Fact`** (`webtyp.com/money`, already published,
   v0.1.3) through the `events.Publisher` this module already receives (`Deps.Publisher`, optional). A cash
   view or, later, an accounting module subscribes to these without `payment_manager` knowing either exists —
   see `money`'s `README.md` for `money.Publish`/`money.Subscribe`.

**Deliberately out of scope — do not invent it:** no `shift_id`/`turno` field on `Payment`. Phase M3
(`veltylabs/cash_manager`, a separate module, not built yet) will derive each shift's totals by correlating
the published facts' `At` timestamp and `payment.CreatedBy` against its own shift records — it does not need
`payment_manager` to know shifts exist. Do not add any field, interface or dependency toward a "shift" or
"cash_manager" concept in this plan.

## Git rules (strict)

- Work on the branch codejob created. Never run `git reset`, `git checkout -- <file>`, `git clean`,
  `git stash` or any command that discards files. Commit often.
- **Never pause to ask permission or to report progress.** When the delivery check passes: commit, push and
  open the pull request. If something still fails after 30 minutes, open the pull request anyway and list in
  it exactly what fails (command and output).
- Do not edit `model_orm.go` by hand: it is generated.

## Tools

```bash
go install webtyp.com/ormc/cmd/ormc@latest
go install webtyp.com/devflow/cmd/gotest@latest
```

## Stage 0 — dependency

```bash
go get webtyp.com/money@v0.1.3
go mod tidy
```

## Stage 1 — receipt number generator

Add `receipt.go`:

```go
package paymentmanager

// NextReceiptNumber returns this tenant's next sequential receipt number, formatted "R-000001"
// (zero-padded to 6 digits; grows past it without truncating). It must be gap-free per tenant:
// read the current counter for the tenant inside the same transaction/call that creates the
// payment (follow the pattern this repo already uses for any other per-tenant sequence — read
// AGENTS.md and search the repo for an existing counter before inventing a new storage shape;
// if none exists, store it as a new model.Definition "payment_counter" with fields tenant_id (PK)
// and next_number (model.Int()), read-increment-write under the same orm.DB transaction as the
// payment insert).
func (m *Module) NextReceiptNumber(tenantId string) (string, error)
```

In `CreateCounterPaymentArgs` (model.go) and `UpsertCreateCounterPayment` flow: **delete** the
`ReceiptNumber` field from `CreateCounterPaymentArgs` (it is caller input today — grep every caller in
`ui/`, `tests/`, `seed/` and remove the argument there too). In `module.go`'s `CreateCounterPayment`
(line ~156), replace `ReceiptNumber: cmd.ReceiptNumber` with a call to `m.NextReceiptNumber(cmd.TenantId)`;
on error, return it wrapped, do not silently fall back to an empty receipt number.

## Stage 2 — regenerate and publish facts

```bash
cp go.mod /tmp/x.mod.bak && ormc && cp /tmp/x.mod.bak go.mod && go mod tidy && gofmt -w model_orm.go
```

Add `facts.go`:

```go
package paymentmanager

import "webtyp.com/money"

// paymentFact builds the money.Fact for a successful charge. Scope is the payment's own
// ReferenceModule (e.g. "appointment_booking") — there is no separate business-area vocabulary
// in this module yet; do not invent one.
func paymentFact(p Payment) money.Fact {
	amt, err := money.New(p.Amount, money.Currency(p.Currency))
	if err != nil {
		// Amount/Currency were already validated when the Payment was created; this is
		// unreachable in practice. Return a Fact that Validate() will reject rather than
		// panicking — the publish call below checks the error.
	}
	return money.Fact{
		Kind:   money.Charged,
		Side:   money.Inflow,
		Amount: amt,
		Scope:  p.ReferenceModule,
		Source: money.Ref{Type: "payment", ID: p.Id},
		Party:  p.ReferenceId,
		At:     p.PaidAt,
	}
}

// refundFact builds the money.Fact for a refund.
func refundFact(p Payment, r Refund) money.Fact {
	amt, _ := money.New(r.Amount, money.Currency(p.Currency))
	return money.Fact{
		Kind:   money.Refunded,
		Side:   money.Outflow,
		Amount: amt,
		Scope:  p.ReferenceModule,
		Source: money.Ref{Type: "refund", ID: r.Id},
		Party:  p.ReferenceId,
		At:     r.CreatedAt,
	}
}
```

Call sites (both only when `m.deps.Publisher != nil`, same guard this module already uses for its existing
`publish` calls — read how `module.go` currently calls `m.pub`/`events.Publisher` and follow the same style,
do not add a second mechanism):

- End of `CreateCounterPayment`, after the payment is persisted and status is `StatusPaid`: build
  `paymentFact(p)` and call `money.Publish(m.deps.Publisher, fact)`. A publish error (invalid fact) is a bug
  in this code, not a runtime condition to swallow: log it the same way this module already logs/reports
  publish failures of its own domain events (check `module.go` for the existing pattern); if none exists, return
  the error from `CreateCounterPayment` — never discard it silently (`zero-debt`: no silent fallback).
- Also handle the online flow (`Commit`/webpay) wherever it transitions a Payment to `StatusPaid` — grep
  `StatusPaid` across `module.go` for every assignment site, not only `CreateCounterPayment`.
- End of `RefundPayment`, after the refund is persisted: build `refundFact(p, ref)` and publish it the
  same way.

## Stage 3 — tests

Through the exported API, with `webtyp.com/events/mock`'s Broker (same pattern `webtyp/money`'s own tests
use) and a recording `money.Sink` subscribed via `money.Subscribe`:

- `NextReceiptNumber` returns sequential, gap-free, zero-padded numbers per tenant; two tenants have
  independent sequences; concurrent calls for the same tenant never return the same number (`gotest -race`).
- `CreateCounterPayment` no longer accepts a caller-supplied receipt number (the field is gone from the
  args type — this is a compile-time check, not a runtime test).
- A successful counter payment publishes exactly one `Charged` fact with the right `Amount`/`Scope`/`Source`.
- A refund publishes exactly one `Refunded` fact.
- No `Publisher` configured (`Deps.Publisher == nil`) → no panic, no fact published, the payment/refund still
  succeeds (this module already treats `Publisher` as optional for its own events — keep that contract).

## Stage 4 — docs

`README.md`: receipt numbers are system-generated (remove any claim that the caller provides one); add a
short "Economic facts" section pointing at `webtyp.com/money`'s README for `Publish`/`Subscribe`, mirroring
how another already-migrated module in this ecosystem documents a `money` dependency (if none exists yet,
write it plainly: this module publishes `Charged`/`Refunded` facts, scope is `ReferenceModule`, no accounting
here — `money.Sink` is implemented elsewhere).

## Delivery check

- `gotest` green (vet, race, tests, wasm).
- `grep -rn "ReceiptNumber" --include="*.go" .` outside `model_orm.go`/tests shows no remaining caller input
  path for it (only the new `NextReceiptNumber` and its storage).
- `git diff --stat main --diff-filter=D` empty.
- No behaviour change beyond: receipt numbers are generated, not accepted; facts are published when a
  `Publisher` is configured.

## Stages

| # | File(s) | What |
|---|---|---|
| 0 | `go.mod` | add `webtyp.com/money` |
| 1 | `receipt.go` (new), `model.go`, `module.go`, callers in `ui/`/`seed/` | sequential receipt numbers, delete caller-supplied field |
| 2 | `model_orm.go`, `facts.go` (new), `module.go` | regenerate; publish `money.Fact` on charge/refund |
| 3 | `tests/*` | receipt sequence, fact publication, optional-publisher contract |
| 4 | `README.md` | document generated receipts and the `money` dependency |
