package paymentmanager

import "webtyp.com/crud"

func (p *PaymentReconciliation) Create(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error) {
	p.Status = ReconcilePending
	p.ActorId = a.UserID
	return crud.ApplyAndPublish(PaymentReconciliationReconcile{}), nil
}
