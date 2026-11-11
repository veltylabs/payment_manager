package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/model"
)

func (p *PaymentManualConfirmation) Create(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error) {
	pay, err := m.repo.paymentById(p.TenantId, p.PaymentId)
	if err != nil {
		return crud.RuleAnswer{}, err
	}
	if pay.Channel != ChannelTerminal {
		return crud.RuleAnswer{}, ErrNotTerminal
	}
	if pay.Status != StatusPending {
		return crud.RuleAnswer{}, ErrPaymentNotPending
	}
	if p.Note == "" {
		return crud.RuleAnswer{}, &model.FieldError{Field: "note", Err: ErrMissingArgs}
	}
	p.ActorId = a.UserID
	return crud.ApplyAndPublish(PaymentManualConfirmationConfirm{}), nil
}
