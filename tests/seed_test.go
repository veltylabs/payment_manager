package tests

import (
	"testing"

	"webtyp.com/storage"

	pm "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/seed"
)

func TestSeed_LoadsPaidAndRefunded(t *testing.T) {
	e := newEnv(t)
	data, err := seed.Load(e.m, tenant, []seed.Ref{{Module: refModule, Id: "r1"}, {Module: refModule, Id: "r2"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Payments) != 2 || len(data.Refunds) != 1 {
		t.Fatalf("data = %+v", data)
	}
	if p := e.payment(t, data.Payments[1].Id); p.Status != pm.StatusPartiallyRefunded {
		t.Fatalf("second payment status %s", p.Status)
	}
}

func TestSeed_NoRefsNoData(t *testing.T) {
	data, err := seed.Load(newEnv(t).m, tenant, nil)
	if err != nil || len(data.Payments) != 0 {
		t.Fatalf("data = %+v, %v", data, err)
	}
}

func TestSeed_OneRefOnlyPays(t *testing.T) {
	data, err := seed.Load(newEnv(t).m, tenant, []seed.Ref{{Module: refModule, Id: "r1"}})
	if err != nil || len(data.Payments) != 1 || len(data.Refunds) != 0 {
		t.Fatalf("data = %+v, %v", data, err)
	}
}

func TestSeed_ErrorsPropagate(t *testing.T) {
	if _, err := seed.Load(newEnv(t).m, tenant, []seed.Ref{{Module: "otro", Id: "r1"}}); err != pm.ErrUnknownReference {
		t.Fatalf("first payment err = %v", err)
	}
	if _, err := seed.Load(newEnv(t).m, tenant, []seed.Ref{{Module: refModule, Id: "r1"}, {Module: "otro", Id: "r2"}}); err != pm.ErrUnknownReference {
		t.Fatalf("second payment err = %v", err)
	}
	e := newEnv(t)
	e.conn.failOn(tRefund, storage.ActionCreate, 0)
	if _, err := seed.Load(e.m, tenant, []seed.Ref{{Module: refModule, Id: "r1"}, {Module: refModule, Id: "r2"}}); err == nil {
		t.Fatal("refund error must propagate")
	}
}
