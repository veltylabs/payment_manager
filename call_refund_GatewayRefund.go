package paymentmanager

import "webtyp.com/crud"

func (p *Refund) GatewayRefund(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	pay, err := m.repo.paymentById(p.TenantId, p.PaymentId)
	if err != nil {
		return nil, err
	}
	res, gwErr := m.gw.Refund(pay.ProviderToken, p.Amount)
	if gwErr != nil {
		return nil, ErrGatewayFailed
	}

	if res.Kind == RefundKindReversed {
		p.Kind = RefundKindReversed
	} else if res.Kind == RefundKindNullified {
		p.Kind = RefundKindNullified
	} else {
		p.Kind = RefundKindManual
	}
	p.AuthorizationCode = res.AuthorizationCode
	// payment bookkeeping
	pay.RefundedAmount += p.Amount
	if pay.RefundedAmount >= pay.Amount {
		pay.Status = StatusRefunded
	} else {
		pay.Status = StatusPartiallyRefunded
	}
	if err := m.store.UpdateFields(&pay, pay.Id, []string{"refunded_amount", "status"}); err != nil {
		return nil, err
	}
	m.publish(EventPaymentRefunded, p)

	return crud.Fields("kind", "authorization_code"), nil
}
