package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/model"
)

func (p *OnlinePaymentRequest) Create(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error) {
	if m.gw == nil {
		return crud.RuleAnswer{}, ErrGatewayUnavailable
	}
	if p.Id == "" {
		return crud.RuleAnswer{}, &model.FieldError{Field: "id", Err: ErrMissingArgs}
	}
	// Verify id taken
	_, err := m.repo.paymentById(p.TenantId, p.Id)
	if err == nil {
		return crud.RuleAnswer{}, ErrIdTaken
	}

	amount, currency, err := m.Due(p.TenantId, p.ReferenceModule, p.ReferenceId)
	if err != nil {
		return crud.RuleAnswer{}, err
	}

	pay := Payment{
		Id:              p.Id,
		TenantId:        p.TenantId,
		ReferenceModule: p.ReferenceModule,
		ReferenceId:     p.ReferenceId,
		Channel:         ChannelOnline,
		Method:          MethodCard,
		Amount:          amount,
		Currency:        currency,
		Status:          StatusPending,
		ReturnTo:        p.ReturnUrl,
	}
	if err := tx.Create(&pay); err != nil {
		return crud.RuleAnswer{}, err
	}
	p.PaymentId = p.Id
	p.Status = StatusPending
	return crud.ApplyAndPublish(OnlinePaymentRequestGatewayStart{}), nil
}
