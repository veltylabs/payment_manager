package paymentmanager

import "webtyp.com/crud"

// call_online_payment_request_gateway_start.go — WaitForReply; la respuesta del create lleva lo que se establece aquí
func (p *OnlinePaymentRequest) GatewayStart(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	pay, err := m.repo.paymentById(p.TenantId, p.PaymentId)
	if err != nil {
		return nil, err
	}
	started, err := m.gw.Start(GatewayStart{PaymentId: pay.Id, Amount: pay.Amount, Currency: pay.Currency,
		Payer: Payer{Name: p.PayerName, Email: p.PayerEmail, Phone: p.PayerPhone}, WaitSeconds: m.timeouts.OnlineStartSeconds})
	if err != nil {
		if _, cerr := m.close(pay, EventPaymentFailed); cerr != nil {
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
