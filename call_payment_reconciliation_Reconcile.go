package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/time"
)

func (p *PaymentReconciliation) Reconcile(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	n, err := m.ReconcilePayments(p.TenantId, time.NowUnix())
	if err != nil {
		return nil, err
	}
	p.Reconciled, p.Status = int64(n), ReconcileDone
	return crud.Fields("reconciled", "status"), nil
}
