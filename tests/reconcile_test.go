package tests

import (
	"testing"

	"webtyp.com/storage"
	tm "webtyp.com/time"

	pm "github.com/veltylabs/payment_manager"
)

// later es un "ahora" en que el pago ya superó PendingTimeout.
func later() tm.Unix { return tm.NowUnix() + pm.PendingTimeout + 1 }

func reconcile(t *testing.T, e *env, now tm.Unix) int {
	t.Helper()
	n, err := e.m.ReconcilePayments(tenant, now)
	if err != nil {
		t.Fatalf("ReconcilePayments: %v", err)
	}
	return n
}

func TestReconcile_NoGatewayNothingToDo(t *testing.T) {
	if n := reconcile(t, newEnvNoGateway(t), later()); n != 0 {
		t.Fatalf("closed %d", n)
	}
}

func TestReconcile_RecentPendingUntouched(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	if n := reconcile(t, e, tm.NowUnix()); n != 0 {
		t.Fatalf("closed %d, want 0", n)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusPending {
		t.Fatalf("status %s", p.Status)
	}
}

func TestReconcile_OlderThanWindowExpiresWithoutGateway(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.hook.onStatus = func() { t.Fatal("past the status window the gateway must not be asked") }
	if n := reconcile(t, e, tm.NowUnix()+pm.GatewayStatusWindow+pm.PendingTimeout+1); n != 1 {
		t.Fatalf("closed %d", n)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusExpired {
		t.Fatalf("status %s, want EXPIRED", p.Status)
	}
}

func TestReconcile_GatewayErrorSkipsAndContinues(t *testing.T) {
	e := newEnv(t)
	for _, id := range []string{"p1", "p2"} {
		e.gw.Approve(e.started(t, id, "ref-"+id))
	}
	e.gw.FailNext(errSimulated)
	if n := reconcile(t, e, later()); n != 1 {
		t.Fatalf("closed %d, want 1 (the other is skipped)", n)
	}
	paid := 0
	for _, id := range []string{"p1", "p2"} {
		if e.payment(t, id).Status == pm.StatusPaid {
			paid++
		}
	}
	if paid != 1 {
		t.Fatalf("paid %d, want 1", paid)
	}
}

func TestReconcile_AuthorizedPays(t *testing.T) {
	e := newEnv(t)
	e.gw.Approve(e.started(t, "p1", refID))
	if n := reconcile(t, e, later()); n != 1 {
		t.Fatalf("closed %d", n)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusPaid {
		t.Fatalf("status %s", p.Status)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Fatalf("events %v", topics)
	}
}

func TestReconcile_RejectedFails(t *testing.T) {
	e := newEnv(t)
	e.gw.Reject(e.started(t, "p1", refID))
	reconcile(t, e, later())
	if p := e.payment(t, "p1"); p.Status != pm.StatusFailed {
		t.Fatalf("status %s", p.Status)
	}
}

func TestReconcile_StillPendingExpires(t *testing.T) {
	e := newEnv(t)
	e.started(t, "p1", refID)
	reconcile(t, e, later())
	if p := e.payment(t, "p1"); p.Status != pm.StatusExpired {
		t.Fatalf("status %s", p.Status)
	}
}

func TestReconcile_AmountMismatchCountsAsClosed(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.gw.SetCommitAmount(token, 1000)
	if n := reconcile(t, e, later()); n != 1 {
		t.Fatalf("closed %d", n)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusFailed || e.gw.Refunded(token) != 1000 {
		t.Fatalf("status %s refunded %d", p.Status, e.gw.Refunded(token))
	}
}

func TestReconcile_RefundFailureSkips(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.gw.SetCommitAmount(token, 1000)
	e.hook.refundErr = errSimulated
	if n := reconcile(t, e, later()); n != 0 {
		t.Fatalf("closed %d, want 0 (retried later)", n)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusPending {
		t.Fatalf("status %s", p.Status)
	}
}

func TestReconcile_ConflictSkips(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.hook.onStatus = func() {
		if _, err := e.m.CompleteOnlinePayment(pm.OnlineResult{PaymentId: "p1", Result: pm.GatewayResult{Outcome: pm.OutcomeCancelled}}); err != nil {
			t.Fatalf("concurrent abort: %v", err)
		}
	}
	if n := reconcile(t, e, later()); n != 0 {
		t.Fatalf("closed %d, want 0 (another process closed it)", n)
	}
}

func TestReconcile_ListErrorReturned(t *testing.T) {
	e := newEnv(t)
	e.conn.failOn("payment", storage.ActionReadAll, 0)
	if _, err := e.m.ReconcilePayments(tenant, later()); err != errSimulated {
		t.Fatalf("err = %v", err)
	}
}

func TestReconcile_UpdateErrorReturned(t *testing.T) {
	e := newEnv(t)
	e.started(t, "p1", refID)
	e.conn.failOn("payment", storage.ActionUpdate, 0)
	if _, err := e.m.ReconcilePayments(tenant, later()); err != errSimulated {
		t.Fatalf("err = %v", err)
	}
}

func TestReconcile_TerminalAuthorizedPays(t *testing.T) {
	e := newEnvTerminal(t)
	token := startTerminal(t, e, "p1")
	e.term.Approve(token)
	if n := reconcile(t, e, tm.NowUnix() + pm.PendingTimeout + 1); n != 1 {
		t.Fatalf("closed %d", n)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusPaid {
		t.Fatalf("status %s", p.Status)
	}
}

func TestReconcile_TerminalExpiredWindow(t *testing.T) {
	e := newEnvTerminal(t)
	startTerminal(t, e, "p1")
	if n := reconcile(t, e, tm.NowUnix() + pm.GatewayStatusWindow + pm.PendingTimeout + 1); n != 1 {
		t.Fatalf("closed %d", n)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusExpired {
		t.Fatalf("status %s, want EXPIRED", p.Status)
	}
}
