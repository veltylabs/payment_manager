package tests

import (
	"testing"

	"webtyp.com/storage"

	pm "github.com/veltylabs/payment_manager"
)

const (
	tPayment = "payment"
	tRefund  = "refund"
)

func wantSimulated(t *testing.T, name string, err error) {
	t.Helper()
	if err != errSimulated {
		t.Errorf("%s: err = %v, want the storage error", name, err)
	}
}

func TestStorageErrors_CreateCounterPayment(t *testing.T) {
	cases := []struct {
		name   string
		action storage.Action
		skip   int
	}{
		{"id lookup", storage.ActionReadOne, 0},
		{"reference lookup", storage.ActionReadAll, 0},
		{"insert", storage.ActionCreate, 0},
	}
	for _, c := range cases {
		e := newEnv(t)
		e.conn.failOn(tPayment, c.action, c.skip)
		_, err := e.m.CreateCounterPayment(counterArgs("p1"))
		wantSimulated(t, c.name, err)
	}
}

func TestStorageErrors_StartOnlinePayment(t *testing.T) {
	for _, c := range []struct {
		name   string
		action storage.Action
	}{{"id lookup", storage.ActionReadOne}, {"reference lookup", storage.ActionReadAll}, {"insert", storage.ActionCreate}} {
		e := newEnv(t)
		e.conn.failOn(tPayment, c.action, 0)
		_, err := e.m.StartOnlinePayment(startArgs("p1"))
		wantSimulated(t, c.name, err)
	}
	// insert of the FAILED record after a gateway error
	e := newEnv(t)
	e.gw.FailNext(errSimulated)
	e.conn.failOn(tPayment, storage.ActionCreate, 0)
	_, err := e.m.StartOnlinePayment(startArgs("p1"))
	wantSimulated(t, "insert failed record", err)
}

func TestStorageErrors_CompleteOnlinePayment(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.conn.failOn(tPayment, storage.ActionReadOne, 0)
	_, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantSimulated(t, "token lookup", err)

	e = newEnv(t)
	token = e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.conn.failOn(tPayment, storage.ActionUpdate, 0)
	_, err = e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantSimulated(t, "update", err)

	e = newEnv(t)
	token = e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.gw.SetCommitAmount(token, 1)
	e.conn.failOn(tPayment, storage.ActionUpdate, 0)
	_, err = e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantSimulated(t, "update after amount mismatch", err)

	e = newEnv(t)
	token = e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.conn.failOn(tPayment, storage.ActionReadOne, 1) // the re-read inside the transaction
	_, err = e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantSimulated(t, "re-read in tx", err)
}

func TestStorageErrors_RefundPayment(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	e.conn.failOn(tRefund, storage.ActionReadOne, 0)
	_, err := e.m.RefundPayment(refundArgs("f1", "p1", 100))
	wantSimulated(t, "refund id lookup", err)

	e = newEnv(t)
	e.paidCounter(t, "p1", refID)
	e.conn.failOn(tPayment, storage.ActionReadOne, 0)
	_, err = e.m.RefundPayment(refundArgs("f1", "p1", 100))
	wantSimulated(t, "payment lookup", err)

	e = newEnv(t)
	e.paidCounter(t, "p1", refID)
	e.conn.failOn(tRefund, storage.ActionCreate, 0)
	_, err = e.m.RefundPayment(refundArgs("f1", "p1", 100))
	wantSimulated(t, "insert refund inside tx", err)
	if p := e.payment(t, "p1"); p.RefundedAmount != 0 || p.Revision != 1 {
		t.Fatalf("nothing written expected, got %+v", p)
	}
}

func TestStorageErrors_PaidAmount(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	e.conn.failOn(tPayment, storage.ActionReadOne, 0)
	_, _, err := e.m.PaidAmount(tenant, "p1", refModule, refID)
	wantSimulated(t, "lookup", err)
}

func TestStorageErrors_Reads(t *testing.T) {
	e := newEnv(t)
	e.conn.failOn(tPayment, storage.ActionReadAll, 0)
	_, err := e.m.ListPayments(tenant, 0, 1)
	wantSimulated(t, "ListPayments", err)
	_, err = e.m.ListPaymentsByReference(tenant, refModule, refID)
	wantSimulated(t, "ListPaymentsByReference", err)

	e = newEnv(t)
	e.conn.failOn(tRefund, storage.ActionReadAll, 0)
	_, err = e.m.ListRefunds(tenant, "p1")
	wantSimulated(t, "ListRefunds", err)
}
