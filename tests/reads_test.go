package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestGetPayment(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	if p := e.payment(t, "p1"); p.Id != "p1" {
		t.Fatalf("got %+v", p)
	}
	_, err := e.m.GetPayment(tenant, "nope")
	wantErr(t, err, pm.ErrNotFound)
}

func TestListPaymentsByReference(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	e.paidCounter(t, "p2", "r2")
	list, err := e.m.ListPaymentsByReference(tenant, refModule, refID)
	if err != nil || len(list) != 1 || list[0].Id != "p1" {
		t.Fatalf("got %v, %v", list, err)
	}
}

func TestListPayments_DateRange(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	now := nowSec()
	in, err := e.m.ListPayments(tenant, now-60, now+60)
	if err != nil || len(in) != 1 {
		t.Fatalf("in range: %v, %v", in, err)
	}
	out, err := e.m.ListPayments(tenant, now+60, now+120)
	if err != nil || len(out) != 0 {
		t.Fatalf("out of range: %v, %v", out, err)
	}
}

func TestListRefunds(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	if _, err := e.m.RefundPayment(refundArgs("f1", "p1", 100)); err != nil {
		t.Fatal(err)
	}
	refunds, err := e.m.ListRefunds(tenant, "p1")
	if err != nil || len(refunds) != 1 || refunds[0].Id != "f1" {
		t.Fatalf("got %v, %v", refunds, err)
	}
}
