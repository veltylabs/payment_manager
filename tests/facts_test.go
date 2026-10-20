package tests

import (
	"testing"

	"webtyp.com/events/mock"
	"webtyp.com/money"
	"webtyp.com/orm"
	"webtyp.com/storage/mem"

	pm "github.com/veltylabs/payment_manager"
)

type recordingSink struct {
	facts []money.Fact
}

func (s *recordingSink) Accept(f money.Fact) error {
	s.facts = append(s.facts, f)
	return nil
}

func TestFacts_Published(t *testing.T) {
	broker := &mock.Broker{}
	charge := &fakeCharge{amount: 25000, currency: pm.CurrencyCLP, payable: true}
	conn := &failingConn{Conn: mem.New()}

	deps := pm.Deps{
		Publisher: broker,
		Charges:   []pm.ChargeSource{{Module: refModule, Reader: charge}},
	}
	m, err := pm.New(orm.New(conn), deps)
	if err != nil {
		t.Fatal(err)
	}

	sink := &recordingSink{}
	if err := money.Subscribe(broker, sink, func(err error) { t.Error(err) }); err != nil {
		t.Fatal(err)
	}

	// 1. Successful counter payment publishes exactly one Charged fact
	_, err = m.CreateCounterPayment(pm.CreateCounterPaymentArgs{
		TenantId:        tenant,
		Id:              "p_fact_1",
		ReferenceModule: refModule,
		ReferenceId:     refID,
		Method:          pm.MethodCash,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(sink.facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(sink.facts))
	}
	f1 := sink.facts[0]
	if f1.Kind != money.Charged || f1.Side != money.Inflow || f1.Amount.Amount != 25000 || f1.Scope != refModule || f1.Source.Type != "payment" || f1.Source.ID != "p_fact_1" {
		t.Errorf("unexpected fact: %+v", f1)
	}

	// 2. Refund publishes exactly one Refunded fact
	_, err = m.RefundPayment(pm.RefundPaymentArgs{
		TenantId:  tenant,
		Id:        "r_fact_1",
		PaymentId: "p_fact_1",
		Amount:    10000,
		Reason:    "refund",
		Method:    pm.MethodCash,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(sink.facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(sink.facts))
	}
	f2 := sink.facts[1]
	if f2.Kind != money.Refunded || f2.Side != money.Outflow || f2.Amount.Amount != 10000 || f2.Scope != refModule || f2.Source.Type != "refund" || f2.Source.ID != "r_fact_1" {
		t.Errorf("unexpected fact: %+v", f2)
	}
}

func TestFacts_NoPublisher(t *testing.T) {
	charge := &fakeCharge{amount: 25000, currency: pm.CurrencyCLP, payable: true}
	conn := &failingConn{Conn: mem.New()}

	deps := pm.Deps{
		Charges: []pm.ChargeSource{{Module: refModule, Reader: charge}},
	}
	m, err := pm.New(orm.New(conn), deps)
	if err != nil {
		t.Fatal(err)
	}

	// Must not panic, and payment must succeed
	p, err := m.CreateCounterPayment(pm.CreateCounterPaymentArgs{
		TenantId:        tenant,
		Id:              "p_fact_2",
		ReferenceModule: refModule,
		ReferenceId:     refID,
		Method:          pm.MethodCash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != pm.StatusPaid {
		t.Errorf("expected PAID, got %s", p.Status)
	}

	r, err := m.RefundPayment(pm.RefundPaymentArgs{
		TenantId:  tenant,
		Id:        "r_fact_2",
		PaymentId: "p_fact_2",
		Amount:    10000,
		Reason:    "refund",
		Method:    pm.MethodCash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Id != "r_fact_2" {
		t.Errorf("expected r_fact_2, got %s", r.Id)
	}
}
