← [Stage 2](PLAN_STAGE_2_module_go.md) | Next → [Stage 4](PLAN_STAGE_4_readers_used_by_other_module.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 3 — generate, then write the rules & external calls

1. Delete `ops.go`, `crud.go`, `queries.go`, `model_crud.go`, `model_orm.go` and `definition_crud.go` (those that exist). Run `definitionc`: it writes one `definition_gen.go` (the types, schemas, external call types `PaymentTerminalPush`, `PaymentTerminalStatus`, `OnlinePaymentRequestGatewayStart`, `RefundGatewayRefund`, `PaymentReconciliationReconcile`, and on `Module`, `ModelName()`, `Definition()`, `MountOperations()`) and the skeletons:
   - `rule_<entity>_<verb>.go` for every action declared `AddMyBusinessRule`.
   - `call_<entity>_<call>.go` for every call in `ExternalCalls`.
2. Add `store.go`: for each table of «Tables, stores and rules» that has a Store, `// Store<Entity> is the store of <entity>.` `func (m *Module) Store<Entity>() crud.Store { return m.store }`.
3. Fill each rule skeleton:
   - `Payment.Create`: `counter` as today (status `paid`, `crud.ApplyAndPublish()`); `terminal`: `if m.term == nil { return crud.RuleAnswer{}, ErrTerminalUnavailable }`, the checks of `StartTerminalPayment` (method, device, due amount), `p.Status, p.Provider = StatusPending, m.term.Provider()`, `return crud.ApplyAndPublish(PaymentTerminalPush{}, PaymentTerminalStatus{}), nil`.
   - `PaymentManualConfirmation.Create`: load the payment with `m.repo.paymentById(p.TenantId, p.PaymentId)`; `Channel != ChannelTerminal` → `ErrNotTerminal`; `Status != StatusPending` → `ErrPaymentNotPending`; empty `Note` → `&model.FieldError{Field: "note", Err: ErrMissingArgs}`; `p.ActorId = a.UserID`; `return crud.ApplyAndPublish(PaymentManualConfirmationConfirm{}), nil`.
   - `OnlinePaymentRequest.Create`: the checks of `StartOnlinePayment` (`m.gw == nil` → `ErrGatewayUnavailable`, missing args, id taken); `amount, currency, err := m.Due(...)`; `tx.Create(&Payment{Id: p.Id, TenantId: p.TenantId, ReferenceModule: p.ReferenceModule, ReferenceId: p.ReferenceId, Channel: ChannelOnline, Method: MethodCard, Amount: amount, Currency: currency, Status: StatusPending, ReturnTo: p.ReturnUrl})`; `p.PaymentId, p.Status = p.Id, StatusPending`; `return crud.ApplyAndPublish(OnlinePaymentRequestGatewayStart{}), nil`.
   - `Refund.Create`: the checks of `RefundPayment` (amount ≤ paid), `p.Status = StatusPending`, `return crud.ApplyAndPublish(RefundGatewayRefund{}), nil`.
   - `PaymentReconciliation.Create`: `p.Status, p.ActorId = ReconcilePending, a.UserID`; `return crud.ApplyAndPublish(PaymentReconciliationReconcile{}), nil`.
   - Rejection: `return crud.RuleAnswer{}, err`.
4. Calls (the receiver `p` is the stored row; `TerminalStatus` runs in the background: `a` is empty). `close`, `settleTerminal`, `settle` keep writing the payment with revision control and publishing the money facts exactly as today: **do not move them to `crud`**.

```go
// call_payment_terminal_push.go — WaitForReply
func (p *Payment) TerminalPush(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	src, _ := m.source(p.ReferenceModule)
	pushed, err := m.term.Push(TerminalPush{RequestId: p.Id, Device: src.Device, Amount: p.Amount, Currency: p.Currency, Method: p.Method, WaitSeconds: m.timeouts.TerminalPushSeconds})
	if err != nil {
		return nil, ErrTerminalUnreachable // the payment stays pending with no token: the screen shows «no connection» and offers the manual confirmation
	}
	p.ProviderToken = pushed.Token
	return crud.Fields("provider_token"), nil
}

// call_payment_terminal_status.go — RetryInBackground: polls until the machine answers or the window ends
func (p *Payment) TerminalStatus(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	if p.Status != StatusPending {
		return nil, nil
	}
	if int64(time.NowUnix()-p.CreatedAt) > m.timeouts.TerminalPayWindowSeconds {
		return nil, nil // window over: it stays pending and the manual confirmation is offered
	}
	if p.ProviderToken == "" {
		return nil, ErrTerminalUnreachable
	}
	res, err := m.term.Status(p.ProviderToken)
	if err != nil {
		return nil, ErrTerminalUnreachable
	}
	settled, err := m.settleTerminal(*p, res, false)
	if err != nil {
		return nil, err
	}
	if settled.Status == StatusPending {
		return nil, ErrStillPending
	}
	return nil, nil
}

// call_payment_manual_confirmation_confirm.go — WaitForReply
func (p *PaymentManualConfirmation) Confirm(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	pay, err := m.repo.paymentById(p.TenantId, p.PaymentId)
	if err != nil {
		return nil, err
	}
	if pay.Status != StatusPending {
		return nil, ErrPaymentNotPending
	}
	pay.PaidAt = time.NowUnix()
	_, err = m.close(pay, EventPay)
	return nil, err
}

// call_online_payment_request_gateway_start.go — WaitForReply; the create's response carries what is set here
func (p *OnlinePaymentRequest) GatewayStart(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	pay, err := m.repo.paymentById(p.TenantId, p.PaymentId)
	if err != nil {
		return nil, err
	}
	started, err := m.gw.Start(GatewayStart{PaymentId: pay.Id, Amount: pay.Amount, Currency: pay.Currency,
		Payer: Payer{Name: p.PayerName, Email: p.PayerEmail, Phone: p.PayerPhone}, WaitSeconds: m.timeouts.OnlineStartSeconds})
	if err != nil {
		if _, cerr := m.close(pay, EventFail); cerr != nil {
			return nil, cerr
		}
		return nil, ErrGatewayFailed
	}
	pay.ProviderToken, pay.Provider = started.Token, m.gw.Provider()
	if err := m.store.UpdateFields(&pay, pay.Id, []string{"provider_token", "provider"}); err != nil {
		return nil, err
	}
	p.RedirectUrl, p.Token, p.TokenField, p.Status = started.RedirectURL, started.Token, started.TokenField, StatusStarted
	return crud.Fields("redirect_url", "token", "token_field", "status"), nil
}
```

   `Refund.GatewayRefund`: move into it the gateway call and the refund bookkeeping of `RefundPayment` (`module.go:466-556`); it sets `p.Status` from the gateway's answer and returns `crud.Fields("status")`; on a gateway error it returns `nil, ErrGatewayFailed` (the refund stays `pending`). `PaymentReconciliation.Reconcile`: `n, err := m.ReconcilePayments(p.TenantId, time.NowUnix()); if err != nil { return nil, err }; p.Reconciled, p.Status = int64(n), ReconcileDone; return crud.Fields("reconciled", "status"), nil`. New constants in `const.go`: `StatusStarted = "started"`, `ReconcilePending = "pending"`, `ReconcileDone = "done"` (the module's own `Status*` for payments stay).
Delete every `Op*` constant, `res*`/`Resource*` variable, every `op*` handler, every `view.MountSave`/`view.MountDelete`/`reg.Operation` call and the `webtyp.com/router`, `webtyp.com/rpc` imports that only they used (`grep -rn "reg.Operation\|view.MountSave\|view.MountDelete\|router.Context\|rpc.WriteError" --include=*.go . | grep -v _test` → empty).
