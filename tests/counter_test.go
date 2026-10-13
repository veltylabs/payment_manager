package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func counterArgs(id string) pm.CreateCounterPaymentArgs {
	return pm.CreateCounterPaymentArgs{TenantId: tenant, Id: id, ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodDebit, ReceiptNumber: "B-77"}
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
		p.Method != pm.MethodDebit || p.ReceiptNumber != "B-77" || p.PaidAt == 0 || p.Revision != 1 {
		t.Fatalf("unexpected payment %+v", p)
	}
	if got := e.pub.topics(); len(got) != 1 || got[0] != pm.EventPaymentPaid {
		t.Fatalf("events = %v", got)
	}
}

func TestCounter_RetryReturnsStoredWithoutEvent(t *testing.T) {
	e := newEnv(t)
	first := e.paidCounter(t, "p1", refID)
	again, err := e.m.CreateCounterPayment(pm.CreateCounterPaymentArgs{TenantId: tenant, Id: "p1", ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodCash})
	if err != nil || again.Id != first.Id {
		t.Fatalf("retry = %+v, %v", again, err)
	}
	if len(e.pub.events) != 1 {
		t.Fatalf("a retry must not publish again: %v", e.pub.topics())
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
