package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestTenant_ReadsRefundsAndReconcileAreScoped(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	e.gw.Approve(e.started(t, "p2", "r2"))

	if _, err := e.m.GetPayment("t2", "p1"); err != pm.ErrNotFound {
		t.Errorf("GetPayment from t2: %v", err)
	}
	if list, _ := e.m.ListPaymentsByReference("t2", refModule, refID); len(list) != 0 {
		t.Errorf("ListPaymentsByReference from t2: %v", list)
	}
	if list, _ := e.m.ListPayments("t2", 0, nowSec()+60); len(list) != 0 {
		t.Errorf("ListPayments from t2: %v", list)
	}
	a := refundArgs("f1", "p1", 100)
	a.TenantId = "t2"
	if _, err := e.m.RefundPayment(a); err != pm.ErrNotFound {
		t.Errorf("RefundPayment from t2: %v", err)
	}
	if n, _ := e.m.ReconcileOnlinePayments("t2", later()); n != 0 {
		t.Errorf("Reconcile from t2 closed %d", n)
	}
	if p := e.payment(t, "p2"); p.Status != pm.StatusPending {
		t.Errorf("t1 payment touched by t2 reconcile: %s", p.Status)
	}
}
