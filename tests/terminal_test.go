package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/sandbox"
)

func startTerminalArgs(id string) pm.StartTerminalPaymentArgs {
	return pm.StartTerminalPaymentArgs{
		TenantId:        tenant,
		Id:              id,
		ReferenceModule: refModule,
		ReferenceId:     refID,
		Method:          pm.MethodDebit,
	}
}

func startTerminal(t *testing.T, e *env, id string) string {
	t.Helper()
	p, err := e.m.StartTerminalPayment(startTerminalArgs(id))
	if err != nil {
		t.Fatalf("StartTerminalPayment: %v", err)
	}
	return p.ProviderToken
}

func refreshTerminal(t *testing.T, e *env, id string) pm.Payment {
	t.Helper()
	p, err := e.m.RefreshTerminalPayment(tenant, id)
	if err != nil {
		t.Fatalf("RefreshTerminalPayment: %v", err)
	}
	return p
}

func refreshTerminalError(t *testing.T, e *env, id string, want error) {
	t.Helper()
	_, err := e.m.RefreshTerminalPayment(tenant, id)
	if err != want {
		t.Fatalf("RefreshTerminalPayment error = %v, want %v", err, want)
	}
}

func TestTerminal_T1_Authorized(t *testing.T) {
	e := newEnvTerminal(t)
	token := startTerminal(t, e, "p1")
	e.term.Approve(token)
	p := refreshTerminal(t, e, "p1")
	if p.Status != pm.StatusPaid {
		t.Errorf("status = %s, want PAID", p.Status)
	}
	if p.Provider != sandbox.TerminalProvider {
		t.Errorf("provider = %s, want %s", p.Provider, sandbox.TerminalProvider)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Errorf("events = %v, want money.fact and payment.paid", topics)
	}
}

func TestTerminal_T2_Rejected(t *testing.T) {
	e := newEnvTerminal(t)
	token := startTerminal(t, e, "p1")
	e.term.Reject(token)
	p := refreshTerminal(t, e, "p1")
	if p.Status != pm.StatusFailed {
		t.Errorf("status = %s, want FAILED", p.Status)
	}
	topics := e.pub.topics()
	if len(topics) != 1 || topics[0] != pm.EventPaymentFailed {
		t.Errorf("events = %v, want payment.failed", topics)
	}
}

func TestTerminal_T3_Cancelled(t *testing.T) {
	e := newEnvTerminal(t)
	token := startTerminal(t, e, "p1")
	e.term.Cancel(token)
	p := refreshTerminal(t, e, "p1")
	if p.Status != pm.StatusCancelled {
		t.Errorf("status = %s, want CANCELLED", p.Status)
	}
}

func TestTerminal_T4_Pending(t *testing.T) {
	e := newEnvTerminal(t)
	startTerminal(t, e, "p1")
	p1 := e.payment(t, "p1")
	p2 := refreshTerminal(t, e, "p1")
	if p2.Status != pm.StatusPending {
		t.Errorf("status = %s, want PENDING", p2.Status)
	}
	if p1.Revision != p2.Revision {
		t.Errorf("revision changed from %d to %d", p1.Revision, p2.Revision)
	}
}

func TestTerminal_T5_AmountMismatch(t *testing.T) {
	e := newEnvTerminal(t)
	token := startTerminal(t, e, "p1")
	e.term.Approve(token)
	e.term.SetAmount(token, 24000)
	refreshTerminalError(t, e, "p1", pm.ErrAmountMismatch)
	p := e.payment(t, "p1")
	if p.Status != pm.StatusFailed {
		t.Errorf("status = %s, want FAILED", p.Status)
	}
}

func TestTerminal_T6_PushError(t *testing.T) {
	e := newEnvTerminal(t)
	e.term.FailNext(errSimulated)
	_, err := e.m.StartTerminalPayment(startTerminalArgs("p1"))
	if err != pm.ErrTerminalFailed {
		t.Errorf("error = %v, want ErrTerminalFailed", err)
	}
	p := e.payment(t, "p1")
	if p.Status != pm.StatusFailed {
		t.Errorf("status = %s, want FAILED", p.Status)
	}
}

func TestTerminal_T7_StatusError(t *testing.T) {
	e := newEnvTerminal(t)
	startTerminal(t, e, "p1")
	e.term.FailNext(errSimulated)
	refreshTerminalError(t, e, "p1", pm.ErrTerminalFailed)
	p := e.payment(t, "p1")
	if p.Status != pm.StatusPending {
		t.Errorf("status = %s, want PENDING", p.Status)
	}
}

func TestTerminal_T8_CounterDebitWithDevice(t *testing.T) {
	e := newEnvTerminal(t)
	_, err := e.m.CreateCounterPayment(pm.CreateCounterPaymentArgs{
		TenantId:        tenant,
		Id:              "p1",
		ReferenceModule: refModule,
		ReferenceId:     refID,
		Method:          pm.MethodDebit,
	})
	if err != pm.ErrTerminalRequired {
		t.Errorf("error = %v, want ErrTerminalRequired", err)
	}
}

func TestTerminal_T9_SameIdTwice(t *testing.T) {
	e := newEnvTerminal(t)
	startTerminal(t, e, "p1")
	startTerminal(t, e, "p1") // idempotent call
	if pushes := e.term.Pushes(); pushes != 1 {
		t.Errorf("machine pushes = %d, want 1", pushes)
	}
}

func TestTerminal_T10_RefreshCounterPayment(t *testing.T) {
	e := newEnvTerminal(t)
	// We need a counter payment. Since T8 prevents Debit/Credit when device is present,
	// we will create a Cash payment.
	e.paidCounter(t, "p1", refID)
	refreshTerminalError(t, e, "p1", pm.ErrNotTerminal)
}
