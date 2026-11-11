package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/model"
)

func (p *Refund) Create(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error) {
	pay, err := m.repo.paymentById(p.TenantId, p.PaymentId)
	if err != nil {
		return crud.RuleAnswer{}, err
	}
	if pay.Status != StatusPaid && pay.Status != StatusPartiallyRefunded {
		return crud.RuleAnswer{}, ErrNotPaid
	}
	left := pay.Amount - pay.RefundedAmount
	if p.Amount > left {
		return crud.RuleAnswer{}, &model.FieldError{Field: "amount", Err: ErrRefundExceeds}
	}

	p.Method = pay.Method
	p.CreatedBy = a.UserID

	if m.gw == nil {
		p.Kind = RefundKindManual
		pay.RefundedAmount += p.Amount
		if pay.RefundedAmount >= pay.Amount {
			pay.Status = StatusRefunded
		} else {
			pay.Status = StatusPartiallyRefunded
		}
		if err := tx.UpdateFields(&pay, pay.Id, []string{"refunded_amount", "status"}); err != nil {
			return crud.RuleAnswer{}, err
		}
		return crud.ApplyAndPublish(), nil
	}

	return crud.ApplyAndPublish(RefundGatewayRefund{}), nil
}
