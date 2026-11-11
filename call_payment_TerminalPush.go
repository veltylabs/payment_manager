package paymentmanager

import "webtyp.com/crud"

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
