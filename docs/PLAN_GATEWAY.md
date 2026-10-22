---
PLAN: "feat!: processor-agnostic online Gateway (adapter verifies, module settles)"
TAG: v0.3.0
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.

<!-- Parked: becomes docs/PLAN.md when the v0.2.0 plan (card terminal + balance) is closed. Before
dispatch, re-read module.go/ports.go at v0.2.0 and fix any line reference below. -->

# Plan — one online `Gateway` for every processor

## Why

The online port is shaped after Webpay: `Gateway.Commit(token)` is a Webpay step, the module decodes
Webpay's return (`OnlineReturn{Token, AbortToken, BuyOrder}`) and `Deps.ReturnURL` is Webpay's return
route. TUU's online payment (Payment Intent) has no commit: it signs a server-to-server notice. To make
changing the processor a configuration change, the **adapter** verifies whatever its processor sends
(Webpay: calls its commit; TUU: checks the HMAC signature) and hands the module an already verified
result. The module only settles it. Both adapters (`transbank`, `tuu`) are built against this contract;
which one is active is configuration.

Language: identifiers, keys and error strings in English; **comments and docs in Spanish**.

## Delivery rules

- Never pause to ask permission or to report progress. When the Delivery check passes: commit, push,
  open the PR. Stuck for 30 minutes on one point: open the PR anyway, listing what fails.
- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never edit `model_orm.go` by hand: regenerate it with `ormc`.
- Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`; `go mod tidy` before `ormc`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/ormc/cmd/ormc@latest
go install webtyp.com/lang/cmd/langc@latest
```

## Ecosystem search

Only `payment_manager.Gateway` models online payment in `webtyp/*`/`veltylabs/*`; its one adapter is
`github.com/veltylabs/transbank`. **Extend** (reshape) it here; no new repo.

## Design gate

1. **Prior art.** Stripe Checkout (create session → redirect → *webhook* is the source of truth),
   Adyen (redirect + notification webhook, HMAC-signed), Mercado Pago Checkout Pro (preference →
   redirect → IPN). All three: the provider-specific verification (signature or confirm call) lives in the
   integration code, the order system receives "order X: paid/failed". Webpay's commit is one such
   verification.
2. **Novice names.** `OnlineResult` ("the result of an online payment"), `Payer` ("who pays"),
   `GatewayStarted.TokenField` ("form field that carries the token; empty = plain redirect"),
   `OutcomeCancelled`, `OutcomeExpired`.
3. **Complexity ledger.** Concepts −1 (`OnlineReturn` and its three Webpay fields) +1 (`OnlineResult`) ·
   Lines in a new adapter to plug in: one route + `Start/Status/Refund` · Ways to complete an online
   payment: 1 (`CompleteOnlinePayment(OnlineResult)`).
4. **Where it belongs.** Verification is processor knowledge → adapter. Settling, idempotency, amount check,
   facts → this module, unchanged.
5. **What it deletes.** `Gateway.Commit`, `OnlineReturn` (+ `Token`, `AbortToken`, `BuyOrder`),
   `Deps.ReturnURL`, `GatewayStart.BuyOrder`, `GatewayStart.SessionId`, `GatewayStart.ReturnURL`,
   `repository.paymentByProviderToken` (if unused after the change), `ui.FieldToken`, the constant
   `MethodWebpay` (replaced by `MethodCard`: the method of an online card payment, whatever the processor).

## Stage 1 — Contract (`ports.go`, `model.go`, `errors.go`)

```go
// Gateway es la pasarela de pago en línea. La implementación (github.com/veltylabs/transbank,
// github.com/veltylabs/tuu) la inyecta la app; verifica sola lo que le devuelve su procesador y llama a
// CompleteOnlinePayment con el resultado ya verificado.
type Gateway interface {
	Provider() string
	Start(req GatewayStart) (GatewayStarted, error)
	Status(token string) (GatewayResult, error)
	Refund(token string, amount int64) (GatewayRefund, error)
}

type GatewayStart struct {
	PaymentId   string // nuestro id (Webpay: buy_order y session_id; TUU: x_reference)
	Amount      int64
	Currency    string
	Description string // texto corto para el pagador; puede ir vacío
	Payer       Payer  // lo exige TUU; Webpay lo ignora
}

// Payer es quien paga en línea. Los campos pueden ir vacíos; un adaptador que los exige responde error.
type Payer struct {
	Name  string
	Email string
	Phone string
}

type GatewayStarted struct {
	Token       string // id del cobro en el procesador; se guarda en Payment.ProviderToken
	RedirectURL string
	TokenField  string // no vacío: el navegador envía Token por POST en este campo (Webpay: "token_ws"); vacío: GET a RedirectURL
}

// OnlineResult es lo que el adaptador ya verificó (firma del aviso o confirmación con el procesador).
type OnlineResult struct {
	PaymentId string
	Result    GatewayResult
}
```

- Add `OutcomeExpired = "expired"` (Stage 2 of v0.2.0 already added `OutcomeCancelled`).
- `MethodWebpay` → `MethodCard = "card"` (online card payment). Grep every site.
- `StartOnlinePaymentArgsModel` gains optional `payer_name` (Text, max 120), `payer_email` (Text, max 120,
  `Extra: []rune{'@','.','-','_','+'}`, letters+numbers), `payer_phone` (Text, max 20, numbers + `'+'`).
- `StartOnlinePaymentResultModel` gains `token_field` (Text).
- Delete `OnlineReturn`, `Deps.ReturnURL` and the `New` check that requires it.

## Stage 2 — Module (`module.go`, `repository.go`)

- `StartOnlinePayment`: `m.gw.Start(GatewayStart{PaymentId: p.Id, Amount: amount, Currency: currency, Payer: Payer{Name: cmd.PayerName, Email: cmd.PayerEmail, Phone: cmd.PayerPhone}})`;
  `Method: MethodCard`, `Provider: m.gw.Provider()`; result returns `TokenField: started.TokenField`.
- `CompleteOnlinePayment(r OnlineResult) (Payment, error)`:
  1. `m.gw == nil` → `ErrGatewayUnavailable`; `r.PaymentId == ""` → `ErrMissingArgs`.
  2. `p, err := m.repo.paymentAnyTenant(r.PaymentId)`; err → return it.
  3. `p.Status != StatusPending` → return `p, nil` (idempotent).
  4. `return m.settle(p, r.Result, false)`.
- `settle`: add `case OutcomeCancelled: return m.close(p, EventCancel)` and
  `case OutcomeExpired: return m.close(p, EventExpire)`. Nothing else changes (amount mismatch still
  refunds through `m.gw.Refund`).

| Case | Input | Result |
|---|---|---|
| G1 | PENDING, `authorized` same amount | PAID, facts published, `Provider == "sandbox"` |
| G2 | `rejected` | FAILED |
| G3 | `cancelled` | CANCELLED |
| G4 | `expired` | EXPIRED |
| G5 | `pending` | stays PENDING |
| G6 | `authorized` other amount | refund called, FAILED, `ErrAmountMismatch` |
| G7 | payment already PAID | returned unchanged, no event |
| G8 | unknown PaymentId | `ErrNotFound` |
| G9 | empty PaymentId | `ErrMissingArgs` |

## Stage 3 — Sandbox and screen (`sandbox/sandbox.go`, `ui/paymentstep.go`)

- `sandbox.Gateway`: delete `Commit`; `Start` uses `req.PaymentId` (token = `TokenPrefix + PaymentId`),
  `TokenField` = `"token_ws"`. New helper for tests and the demo:
  `func (g *Gateway) Result(token string) (paymentmanager.OnlineResult, error)` — returns the
  `OnlineResult` for that token with the outcome set by `Approve`/`Reject` (default pending) and the
  `charged` amount. Unknown token → `ErrUnknownToken`.
- `ui.PaymentStep.startOnline`: `if out.TokenField != "" { NavigatePost(out.RedirectUrl, []fmt.KeyValue{{Key: out.TokenField, Value: out.Token}}) } else { Navigate(out.RedirectUrl) }`;
  delete `FieldToken`. `Navigate(url string)` is `webtyp.com/dom` ≥ v0.13.27 (`go get webtyp.com/dom@v0.13.27`):
  it opens url with a plain GET.
- Button text `"Pay with Webpay"` → `"Pay online"` (`lang.json`: `"Pagar en línea"`); `langc sync .`.

## Stage 4 — Tests and docs

- `tests/online_complete_test.go`: rewrite with cases G1–G9 using `e.gw.Result(token)`; `tests/setup_test.go`:
  `paidOnline` = `started` + `Approve` + `CompleteOnlinePayment(e.gw.Result(token))`; delete the
  `hookGateway.Commit` hook and `commitResult` (feed a crafted `OnlineResult` instead).
- `README.md`: wiring without `ReturnURL`; "the adapter mounts its own public route and calls
  `CompleteOnlinePayment`". `docs/ARCHITECTURE.md` § Flujo online: rewrite generically (no Webpay names) with
  the two families: confirm call (Webpay) and signed notice (TUU). `docs/diagrams/flows.md` §5 and §9 rewritten.

## Deletions — each must grep empty

```bash
grep -rn "OnlineReturn\|AbortToken\|ReturnURL\|BuyOrder\|SessionId\|MethodWebpay\|FieldToken\|Commit(" --include=*.go --include=*.md .
```

## Delivery check

```bash
go mod tidy && ormc && langc sync . && gotest
```

## Consumers to adapt after this tag (not in this plan)

`github.com/veltylabs/transbank` (commit inside its return route, builds `OnlineResult`, owns its return
URL), `github.com/veltylabs/tuu` (new online adapter), `appointment_booking/web/browser.go` (drops
`ReturnURL`).

## Stages

| # | What | Files |
|---|---|---|
| 1 | Contract | `ports.go`, `model.go`, `model_orm.go` (generated), `errors.go` |
| 2 | Module | `module.go`, `repository.go` |
| 3 | Sandbox and screen | `sandbox/sandbox.go`, `ui/paymentstep.go`, `lang.json` |
| 4 | Tests and docs | `tests/*`, `README.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/flows.md` |
