package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func counterArgs(id string) pm.CreateCounterPaymentArgs {
	return pm.CreateCounterPaymentArgs{TenantId: tenant, Id: id, ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodDebit}
}

func TestNextReceiptNumber(t *testing.T) {
	e := newEnv(t)
	n1, err := e.m.NextReceiptNumber(tenant)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != "R-000001" {
		t.Errorf("got %q, want R-000001", n1)
	}
	n2, _ := e.m.NextReceiptNumber(tenant)
	if n2 != "R-000002" {
		t.Errorf("got %q, want R-000002", n2)
	}
	// Verify per-tenant sequence isolation
	n1_other, _ := e.m.NextReceiptNumber("other_tenant")
	if n1_other != "R-000001" {
		t.Errorf("got %q for other tenant, want R-000001", n1_other)
	}
	// Verify race safety
	ch := make(chan string, 100)
	for i := 0; i < 100; i++ {
		go func() {
			n, _ := e.m.NextReceiptNumber("race_tenant")
			ch <- n
		}()
	}
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		seen[<-ch] = true
	}
	if len(seen) != 100 {
		t.Errorf("got %d unique receipt numbers, want 100", len(seen))
	}
}

func TestCounter_MissingArgs(t *testing.T) {
	e := newEnv(t)
	for _, mutate := range []func(*pm.CreateCounterPaymentArgs){
		func(a *pm.CreateCounterPaymentArgs) { a.TenantId = "" },
		func(a *pm.CreateCounterPaymentArgs) { a.Id = "" },
		func(a *pm.CreateCounterPaymentArgs) { a.ReferenceModule = "" },
		func(a *pm.CreateCounterPaymentArgs) { a.ReferenceId = "" },
		func(a *pm.CreateCounterPaymentArgs) { a.Method = "" },
	} {
		a := counterArgs("p1")
		mutate(&a)
		_, err := e.m.CreateCounterPayment(a)
		wantErr(t, err, pm.ErrMissingArgs)
	}
}

func TestCounter_CreatesPaidAndPublishes(t *testing.T) {
	e := newEnv(t)
	p, err := e.m.CreateCounterPayment(counterArgs("p1"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != pm.StatusPaid || p.Channel != pm.ChannelCounter || p.Amount != 25000 || p.Currency != pm.CurrencyCLP ||
		p.Method != pm.MethodDebit || p.ReceiptNumber != "R-000001" || p.PaidAt == 0 || p.Revision != 1 {
		t.Fatalf("unexpected payment %+v", p)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Fatalf("events = %v", topics)
	}
}

func TestCounter_RetryReturnsStoredWithoutEvent(t *testing.T) {
	e := newEnv(t)
	first := e.paidCounter(t, "p1", refID)
	again, err := e.m.CreateCounterPayment(pm.CreateCounterPaymentArgs{TenantId: tenant, Id: "p1", ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodCash})
	if err != nil || again.Id != first.Id {
		t.Fatalf("retry = %+v, %v", again, err)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Fatalf("a retry must not publish again: %v", topics)
	}
}

func TestCounter_IdTakenByOtherReference(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	a := counterArgs("p1")
	a.ReferenceId = "r2"
	_, err := e.m.CreateCounterPayment(a)
	wantErr(t, err, pm.ErrIdTaken)
}

func TestCounter_IdTakenByOtherTenant(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	a := counterArgs("p1")
	a.TenantId = "t2"
	_, err := e.m.CreateCounterPayment(a)
	wantErr(t, err, pm.ErrIdTaken)
}

func TestCounter_WebpayMethodRejected(t *testing.T) {
	a := counterArgs("p1")
	a.Method = pm.MethodWebpay
	_, err := newEnv(t).m.CreateCounterPayment(a)
	wantErr(t, err, pm.ErrInvalidMethod)
}

func TestCounter_AlreadyPaid(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	_, err := e.m.CreateCounterPayment(counterArgs("p2"))
	wantErr(t, err, pm.ErrAlreadyPaid)
}

func TestCounter_NotPayable(t *testing.T) {
	e := newEnv(t)
	e.charge.payable = false
	_, err := e.m.CreateCounterPayment(counterArgs("p1"))
	wantErr(t, err, pm.ErrNotPayable)
}

func TestCounter_ChargeErrorPropagates(t *testing.T) {
	_, err := newEnv(t).m.CreateCounterPayment(pm.CreateCounterPaymentArgs{TenantId: tenant, Id: "p1", ReferenceModule: "otro", ReferenceId: refID, Method: pm.MethodCash})
	wantErr(t, err, pm.ErrUnknownReference)
}

func TestCounter_InvalidStoredFieldsRejected(t *testing.T) {
	a := counterArgs("id con espacios")
	_, err := newEnv(t).m.CreateCounterPayment(a)
	if err == nil {
		t.Fatal("an id outside the permitted characters must fail validation")
	}
}
