package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
	"webtyp.com/events/mock"
	"webtyp.com/money"
	"webtyp.com/orm"
	"webtyp.com/storage/mem"
	"webtyp.com/time"
)

type recordingSink struct {
	facts []money.Fact
}

func (s *recordingSink) Accept(f money.Fact) error {
	s.facts = append(s.facts, f)
	return nil
}

type mockBankDays struct{}

func (m mockBankDays) BankDayAfter(from time.Unix, days int) (time.Unix, error) {
	// Fixed date for testing F4/F5
	return time.Unix(1776038400), nil // Tue 2026-10-13 00:00:00 UTC
}

type termsAdapter struct {
	pm.Terminal
	pm.Gateway
}

func (t *termsAdapter) Payout(c pm.PayoutCharge) (pm.Payout, error) {
	return pm.Payout{Fee: 447, FeeTax: 85, BusinessDays: 2}, nil
}

func (t *termsAdapter) Provider() string { return "test_provider" }

type dummyTerminal struct{}
func (t dummyTerminal) Provider() string { return "dummy_term" }
func (t dummyTerminal) Push(req pm.TerminalPush) (pm.TerminalPushed, error) {
	return pm.TerminalPushed{Token: "tk"}, nil
}
func (t dummyTerminal) Status(token string) (pm.TerminalResult, error) {
	return pm.TerminalResult{Outcome: pm.OutcomeAuthorized, Amount: 30000}, nil
}

type payoutTerminal struct {
	dummyTerminal
}

func (t *payoutTerminal) Payout(c pm.PayoutCharge) (pm.Payout, error) {
	return pm.Payout{Fee: 447, FeeTax: 85, BusinessDays: 2}, nil
}

func TestFacts_Published(t *testing.T) {
	broker := &mock.Broker{}
	charge := &fakeCharge{amount: 25000, currency: pm.CurrencyCLP, payable: true}
	conn := &failingConn{Conn: mem.New()}
	term := &dummyTerminal{}

	deps := pm.Deps{
		Publisher: broker,
		Terminal: term,
		Charges:   []pm.ChargeSource{{Module: refModule, Reader: charge, Device: "term_1"}},
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
	if f1.Kind != money.Charged || f1.Side != money.Inflow || f1.Amount.Amount != 25000 || f1.Scope != refModule || f1.Source.Type != "payment" || f1.Source.ID != "p_fact_1" || f1.Means != money.MeansCash {
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
	if f2.Kind != money.Refunded || f2.Side != money.Outflow || f2.Amount.Amount != 10000 || f2.Scope != refModule || f2.Source.Type != "refund" || f2.Source.ID != "r_fact_1" || f2.Means != money.MeansCash {
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

func TestPaymentFacts_Payout(t *testing.T) {
	broker := &mock.Broker{}
	charge := &fakeCharge{amount: 30000, currency: pm.CurrencyCLP, payable: true}
	conn := &failingConn{Conn: mem.New()}
	term := &payoutTerminal{}

	deps := pm.Deps{
		Publisher: broker,
		Terminal: term,
		BankDays: mockBankDays{},
		Charges:   []pm.ChargeSource{{Module: refModule, Reader: charge, Device: "term_1"}},
	}
	m, err := pm.New(orm.New(conn), deps)
	if err != nil {
		t.Fatal(err)
	}

	sink := &recordingSink{}
	if err := money.Subscribe(broker, sink, func(err error) { t.Error(err) }); err != nil {
		t.Fatal(err)
	}

	_, err = m.StartTerminalPayment(pm.StartTerminalPaymentArgs{
		TenantId:        tenant,
		Id:              "p_fact_3",
		ReferenceModule: refModule,
		ReferenceId:     refID,
		Method:          pm.MethodDebit,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Trigger settle terminal
	_, err = m.RefreshTerminalPayment(tenant, "p_fact_3")
	if err != nil {
		t.Fatal(err)
	}

	if len(sink.facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(sink.facts))
	}

	f1 := sink.facts[0]
	if f1.Kind != money.Accrued || f1.Side != money.Inflow || f1.Amount.Amount != 30000 || f1.Scope != refModule || f1.Source.Type != "payment" || f1.Source.ID != "p_fact_3" || f1.Means != money.MeansCard || f1.DueAt != time.Unix(1776038400) {
		t.Errorf("unexpected fact: %+v", f1)
	}

	f2 := sink.facts[1]
	if f2.Kind != money.Accrued || f2.Side != money.Outflow || f2.Amount.Amount != 532 || f2.Scope != refModule || f2.Source.Type != "payment_fee" || f2.Source.ID != "p_fact_3" || f2.Means != money.MeansCard || f2.DueAt != time.Unix(1776038400) {
		t.Errorf("unexpected fact: %+v", f2)
	}
}

func TestPaymentFacts_MissingBankDays(t *testing.T) {
	charge := &fakeCharge{amount: 30000, currency: pm.CurrencyCLP, payable: true}
	conn := &failingConn{Conn: mem.New()}
	term := &payoutTerminal{}

	deps := pm.Deps{
		Terminal: term,
		Charges:   []pm.ChargeSource{{Module: refModule, Reader: charge, Device: "term_1"}},
	}
	_, err := pm.New(orm.New(conn), deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "payment_manager: Deps.BankDays is required when an adapter implements PayoutTerms" {
		t.Errorf("unexpected error message: %v", err)
	}
}
