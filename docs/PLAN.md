---
PLAN: "feat!: processor-agnostic online Gateway (adapter verifies, module settles)"
TAG: v0.3.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 2612195174648171326
---

> Read AGENTS.md at the repo root before any change.

<!-- Parked. Verified against payment_manager v0.2.0 on 2026-10-22. Becomes docs/PLAN.md when dispatched. -->

# Plan — one online `Gateway` for every processor

## Why

The online port is shaped after Webpay: `Gateway.Commit(token)` is a Webpay step, the module decodes Webpay's
return (`OnlineReturn{Token, AbortToken, BuyOrder}`) and `Deps.ReturnURL` is Webpay's return route. TUU's online
payment (Payment Intent) has no commit: it signs a server-to-server notice. To make the processor a configuration
choice, the **adapter** verifies whatever its processor sends (Webpay: its commit call; TUU: the HMAC signature) and
hands the module an already verified result; the module only settles it. Both adapters (`transbank`, `tuu`) are
built against this contract.

Language: identifiers, keys and error strings in English; **comments and docs in Spanish**.

## Delivery rules

- **Commit after every stage.** Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash`.
- Never pause to ask permission or to report progress. When the Delivery check passes: push and open the PR. Stuck
  for 30 minutes: open the PR anyway, listing what fails.
- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never edit `model_orm.go` by hand: regenerate it with `ormc`.
- The code blocks below are the implementation: copy them; do not re-derive them.

## Tools and pinned versions

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/ormc/cmd/ormc@latest
go install webtyp.com/lang/cmd/langc@latest
export GOPRIVATE='webtyp.com/*,github.com/veltylabs/*' GOFLAGS=-mod=mod
go get webtyp.com/dom@v0.13.27 && go mod tidy
```

`dom` v0.13.27 adds `dom.Navigate(url string)` (leaves the page with a plain GET).

## Ecosystem search

Only `payment_manager.Gateway` models online payment in `webtyp/*`/`veltylabs/*`; its adapter is
`github.com/veltylabs/transbank`. **Extend** (reshape) it here.

## Design gate

1. **Prior art.** Stripe Checkout (session → redirect → webhook is the truth), Adyen (redirect + HMAC-signed
   notification), Mercado Pago Checkout Pro (preference → redirect → IPN): provider-specific verification lives in
   the integration, the order system receives "order X: paid/failed".
2. **Names.** `OnlineResult`, `Payer`, `GatewayStarted.TokenField`, `OutcomeExpired`, `MethodCard`.
3. **Ledger.** −1 concept (`OnlineReturn` and its three Webpay fields) +1 (`OnlineResult`); one way to complete an
   online payment.
4. **Where.** Verification → adapter. Settling, idempotency, amount check, facts → this module, unchanged.
5. **Deletes.** `Gateway.Commit`, `OnlineReturn`, `Deps.ReturnURL` (+ its check in `New` and `retURL`),
   `GatewayStart.BuyOrder/SessionId/ReturnURL`, `repository.paymentByProviderToken`, `ui.FieldToken`, `MethodWebpay`.

## Stage 1 — Contract (`ports.go`, `model.go`)

In `ports.go`, the comment that today sits above `Terminal` but describes `Gateway` moves to `Gateway`. `Gateway`
and its types become:

```go
// Gateway es la pasarela de pago en línea. La implementación (github.com/veltylabs/transbank,
// github.com/veltylabs/tuu) la inyecta la app; verifica sola lo que le devuelve su procesador y llama a
// CompleteOnlinePayment con el resultado ya verificado. Sus llamadas son síncronas y solo corren en el servidor.
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

- Add `OutcomeExpired = "expired"` to the `Outcome*` block.
- Delete `OnlineReturn`.
- `model.go`: `MethodWebpay = "webpay"` → `MethodCard = "card" // pago en línea con tarjeta, cualquiera sea el procesador`.
- `StartOnlinePaymentArgsModel` gains, after `return_to`:

```go
		{Name: "payer_name", Type: model.Text(), Permitted: model.Permitted{Letters: true, Tilde: true, Spaces: true, Maximum: 120}},
		{Name: "payer_email", Type: model.Text(), Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'@', '.', '-', '_', '+'}, Maximum: 120}},
		{Name: "payer_phone", Type: model.Text(), Permitted: model.Permitted{Numbers: true, Extra: []rune{'+'}, Maximum: 20}},
```

- `StartOnlinePaymentResultModel` gains `{Name: "token_field", Type: model.Text()},`.

Run `go mod tidy && ormc`. **Commit.**

## Stage 2 — Module (`module.go`, `repository.go`)

- `Deps`: delete `ReturnURL`. `New`: delete the `ReturnURL` check. `Module`: delete `retURL`.
- `StartOnlinePayment`: the payment gets `Method: MethodCard` (it already sets `Provider: m.gw.Provider()`); the
  call becomes

```go
	started, gwErr := m.gw.Start(GatewayStart{
		PaymentId: p.Id,
		Amount:    amount,
		Currency:  currency,
		Payer:     Payer{Name: cmd.PayerName, Email: cmd.PayerEmail, Phone: cmd.PayerPhone},
	})
```

  and the result returns `StartOnlinePaymentResult{PaymentId: p.Id, RedirectUrl: started.RedirectURL, Token: started.Token, TokenField: started.TokenField}`.
- `CompleteOnlinePayment` is replaced entirely by:

```go
// CompleteOnlinePayment asienta el resultado que el adaptador ya verificó. Es idempotente: un pago que ya no
// está PENDING se devuelve tal cual.
func (m *Module) CompleteOnlinePayment(r OnlineResult) (Payment, error) {
	if m.gw == nil {
		return Payment{}, ErrGatewayUnavailable
	}
	if r.PaymentId == "" {
		return Payment{}, ErrMissingArgs
	}
	p, err := m.repo.paymentAnyTenant(r.PaymentId)
	if err != nil {
		return Payment{}, err
	}
	if p.Status != StatusPending {
		return p, nil
	}
	return m.settle(p, r.Result, false)
}
```

- `settle`: before the final `if fromReconcile`, add

```go
	case OutcomeCancelled:
		return m.close(p, EventCancel)
	case OutcomeExpired:
		return m.close(p, EventExpire)
```

  as two more `case` branches of its `switch result.Outcome`.
- `repository.go`: delete `paymentByProviderToken` (no caller left).

| Case | Input | Result |
|---|---|---|
| G1 | PENDING, `authorized` same amount | PAID, `payment.paid` + money fact, `Provider == "sandbox"`, `Method == card` |
| G2 | `rejected` | FAILED |
| G3 | `cancelled` | CANCELLED |
| G4 | `expired` | EXPIRED |
| G5 | `pending` | stays PENDING |
| G6 | `authorized` other amount | refund called, FAILED, `ErrAmountMismatch` |
| G7 | payment already PAID | returned unchanged, no new event |
| G8 | unknown PaymentId | `ErrNotFound` |
| G9 | empty PaymentId | `ErrMissingArgs` |
| G10 | no gateway | `ErrGatewayUnavailable` |

**Commit.**

## Stage 3 — Sandbox, screen, demo (`sandbox/sandbox.go`, `ui/paymentstep.go`, `web/browser.go`, `lang.json`)

- `sandbox.Gateway`: delete `Commit`. `Start` keys the transaction by `req.PaymentId` (token =
  `TokenPrefix + req.PaymentId`) and returns `TokenField: "token_ws"`. Add:

```go
// Result devuelve, para pruebas y la demo, el OnlineResult de un token: el resultado que fijaron Approve o Reject
// (pendiente si ninguno) y el monto cobrado. Token desconocido → ErrUnknownToken.
func (g *Gateway) Result(token string) (paymentmanager.OnlineResult, error)
```

  (the `PaymentId` is the token without `TokenPrefix`; `Result` = what `Status(token)` returns today).
- `ui/paymentstep.go`: delete `FieldToken`; in `startOnline` replace the `NavigatePost` line with

```go
		if out.TokenField != "" {
			NavigatePost(out.RedirectUrl, []fmt.KeyValue{{Key: out.TokenField, Value: out.Token}})
			return
		}
		Navigate(out.RedirectUrl)
```

  and the button text `textPayWithWebpay lang.Text = "Pay with Webpay"` → `textPayOnline lang.Text = "Pay online"`.
- `web/browser.go` line 48: delete `ReturnURL: "/",`.
- `lang.json`: run `langc sync .`; delete `"Pay with Webpay"`; set `"Pay online": ["Pagar en línea"]`.

**Commit.**

## Stage 4 — Tests and docs

Every site below is known from `grep -rn "OnlineReturn\|ReturnURL\|MethodWebpay\|FieldToken" tests`:

- `tests/setup_test.go`: drop `deps.ReturnURL = returnURL` and the `returnURL` const; `paidOnline` becomes
  `token := e.started(...); e.gw.Approve(token); r, _ := e.gw.Result(token); e.m.CompleteOnlinePayment(r)`; delete the
  `hookGateway.Commit` hook and `commitResult` (tests that used them build an `OnlineResult` by hand instead).
- `tests/online_complete_test.go`: rewritten as cases G1–G10.
- `tests/reconcile_test.go:140`, `tests/storage_errors_test.go:63–85`: pass `OnlineResult` from `e.gw.Result(token)`
  (abort case: `OnlineResult{PaymentId: id, Result: GatewayResult{Outcome: OutcomeCancelled}}`).
- `tests/module_test.go`: delete `TestNew_GatewayRequiresReturnURL`.
- `tests/models_test.go:32`, `tests/counter_test.go:113`, `tests/refund_test.go:91`, `tests/online_start_test.go:74`:
  `MethodWebpay` → `MethodCard`.
- `tests/ui_browser_test.go:168`: `ui.FieldToken` → `"token_ws"`.
- `README.md`: wiring without `ReturnURL`; "el adaptador monta su propia ruta pública, verifica y llama a
  `CompleteOnlinePayment(OnlineResult)`". `docs/ARCHITECTURE.md` § Flujo online: generic, with the two families
  (confirmación con el procesador — Webpay; aviso firmado — TUU). `docs/diagrams/flows.md` §5 and §9 rewritten.

**Commit.**

## Stage 5 — Spanish (only what is still in English)

This repo is `veltylabs/*`: every `README.md`, `docs/*` file, diagram and code comment is in Spanish, written for a
reader with little technical background (code, identifiers and keys stay English). List the English ones with
`grep -rln "[a-z] the \| is \| and " --include=*.go --include=*.md . | grep -v PLAN` and read each hit; translate
**only** the English text, never rewrite what is already Spanish. Every exported symbol whose comment lacks a usage
example gets one line `// Ejemplo: …`. **Commit.**

## Deletions — each must grep empty

```bash
grep -rn "OnlineReturn\|AbortToken\|ReturnURL\|BuyOrder\|SessionId\|MethodWebpay\|FieldToken\|paymentByProviderToken\|Commit(" --include=*.go --include=*.md --exclude=PLAN*.md .
```

## Delivery check

```bash
go mod tidy && ormc && langc sync . && gotest
```

## Consumers adapted by their own plans after this tag

`github.com/veltylabs/transbank` (`docs/PLAN_GATEWAY_V3.md`), `github.com/veltylabs/tuu`, `appointment_booking/web/browser.go`
(drops `ReturnURL`).

## Stages

| # | What | Files |
|---|---|---|
| 1 | Contract | `ports.go`, `model.go`, `model_orm.go` (generated) |
| 2 | Module | `module.go`, `repository.go` |
| 3 | Sandbox, screen, demo | `sandbox/sandbox.go`, `ui/paymentstep.go`, `web/browser.go`, `lang.json` |
| 4 | Tests and docs | `tests/*`, `README.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/flows.md` |
| 5 | Spanish docs, diagrams, comments + usage examples | every English `*.md` and `*.go` comment |
