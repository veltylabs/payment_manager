package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/time"
)

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
	_, err = m.close(pay, EventPaymentPaid)
	return nil, err
}
