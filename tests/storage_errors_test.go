package tests

import (
	"testing"
	"webtyp.com/orm"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/fmt"
	"webtyp.com/model"
)

type failingConn struct {
	storage.Conn
	failTable  string
	failAction storage.Action
}

func (f *failingConn) Compile(q storage.Query, m model.Model) (storage.Plan, error) {
	if q.Table == f.failTable && q.Action == f.failAction {
		return storage.Plan{}, fmt.Err("error simulado")
	}
	return f.Conn.Compile(q, m)
}

func (f *failingConn) BeginTx() (storage.TxBoundExecutor, error) {
	return f.Conn.(storage.TxExecutor).BeginTx()
}

func setupErrorDB(failTable string, failAction storage.Action) *orm.DB {
	return orm.New(&failingConn{Conn: mem.New(), failTable: failTable, failAction: failAction})
}

func TestStorageErrors_CreateCounterPayment(t *testing.T) {
	db := setupErrorDB("payment", storage.ActionCreate)
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestStorageErrors_StartOnlinePayment(t *testing.T) {
	db := setupErrorDB("payment", storage.ActionCreate)
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: &mockGateway{}, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestStorageErrors_CompleteOnlinePayment(t *testing.T) {
	db := setupErrorDB("payment", storage.ActionReadOne)
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: &mockGateway{}, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok1"})
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestStorageErrors_RefundPayment(t *testing.T) {
	memDb := mem.New()
	ormDb := orm.New(memDb)
	mOk, _ := paymentmanager.New(ormDb, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})
	mOk.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	errDb := orm.New(&failingConn{Conn: memDb, failTable: "refund", failAction: storage.ActionCreate})
	m, _ := paymentmanager.New(errDb, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 200, Reason: "test_reason", Method: paymentmanager.MethodCash})
	if err == nil {
		t.Errorf("expected error during transaction")
	}

	res, _ := mOk.ListRefunds(paymentmanager.ListRefundsArgs{TenantId: "t1", PaymentId: "p1"})
	if len(res) > 0 {
		t.Errorf("expected refund transaction to be rolled back")
	}
}

func TestStorageErrors_ReconcileOnlinePayments(t *testing.T) {
	db := setupErrorDB("payment", storage.ActionReadAll)
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: &mockGateway{}, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.ReconcileOnlinePayments("t1", 0)
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestStorageErrors_PaidAmount(t *testing.T) {
	db := setupErrorDB("payment", storage.ActionReadOne)
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _, err := m.PaidAmount("t1", "p1", "mod", "ref1")
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestStorageErrors_Reads(t *testing.T) {
	db := setupErrorDB("payment", storage.ActionReadOne)
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if err == nil {
		t.Errorf("expected error")
	}

	db2 := setupErrorDB("payment", storage.ActionReadAll)
	m2, _ := paymentmanager.New(db2, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err = m2.ListPaymentsByReference(paymentmanager.ListPaymentsByReferenceArgs{TenantId: "t1", ReferenceModule: "mod", ReferenceId: "ref1"})
	if err == nil {
		t.Errorf("expected error")
	}

	_, err = m2.ListPayments(paymentmanager.ListPaymentsArgs{TenantId: "t1", From: 0, To: 0})
	if err == nil {
		t.Errorf("expected error")
	}

	db3 := setupErrorDB("refund", storage.ActionReadAll)
	m3, _ := paymentmanager.New(db3, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err = m3.ListRefunds(paymentmanager.ListRefundsArgs{TenantId: "t1", PaymentId: "p1"})
	if err == nil {
		t.Errorf("expected error")
	}
}
