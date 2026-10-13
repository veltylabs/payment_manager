package tests

import (
	"webtyp.com/orm"
	"webtyp.com/storage/mem"
	"webtyp.com/events"
)

type mockChargeReader struct {
	amount   int64
	currency string
	payable  bool
	err      error
}

func (m mockChargeReader) Charge(tenantId, referenceId string) (int64, string, bool, error) {
	return m.amount, m.currency, m.payable, m.err
}

type mockPublisher struct {
	events []interface{}
}

func (m *mockPublisher) Publish(e events.Event) {
	m.events = append(m.events, e.Payload)
}

func setupTestDB() *orm.DB {
	return orm.New(mem.New())
}
