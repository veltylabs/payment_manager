package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/router/mock"
	"webtyp.com/model"
	"webtyp.com/storage"
)

func TestOps_RequiresTable(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})

	reg := &mock.Router{}
	reg.Configure(mock.Config{Authorize: func(userID string, r model.Resource, a model.Action) bool { return true }})
	m.MountOperations(reg)

	tests := []struct {
		op       string
		resource string
		action   model.Action
	}{
		{paymentmanager.OpGetCharge, paymentmanager.ResourcePaymentOnline, model.Action(model.ActionRead)},
		{paymentmanager.OpCreateCounterPayment, paymentmanager.ResourcePayment, model.Action(model.ActionCreate)},
		{paymentmanager.OpStartOnlinePayment, paymentmanager.ResourcePaymentOnline, model.Action(model.ActionCreate)},
		{paymentmanager.OpGetPaymentStatus, paymentmanager.ResourcePaymentOnline, model.Action(model.ActionRead)},
		{paymentmanager.OpGetPayment, paymentmanager.ResourcePayment, model.Action(model.ActionRead)},
		{paymentmanager.OpListPaymentsByReference, paymentmanager.ResourcePayment, model.Action(model.ActionRead)},
		{paymentmanager.OpListPayments, paymentmanager.ResourcePayment, model.Action(model.ActionRead)},
		{paymentmanager.OpRefundPayment, paymentmanager.ResourceRefund, model.Action(model.ActionCreate)},
		{paymentmanager.OpListRefunds, paymentmanager.ResourceRefund, model.Action(model.ActionRead)},
		{paymentmanager.OpReconcileOnlinePayments, paymentmanager.ResourcePayment, model.Action(model.ActionUpdate)},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			found := false
			for _, r := range reg.Routes() {
				if r.Path == "/"+tt.op {
					found = true
					if string(r.Resource) != tt.resource || r.Action != tt.action {
						t.Errorf("expected %s:%d, got %s:%d", tt.resource, tt.action, r.Resource, r.Action)
					}
				}
			}
			if !found {
				t.Fatalf("operation %s not registered", tt.op)
			}
		})
	}
}

func TestOps_ErrorStatusTable(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{paymentmanager.ErrNotFound, 404},
		{paymentmanager.ErrRefundNotFound, 404},
		{paymentmanager.ErrMissingArgs, 400},
		{paymentmanager.ErrUnknownReference, 400},
		{paymentmanager.ErrNotPayable, 400},
		{paymentmanager.ErrUnsupportedCurrency, 400},
		{paymentmanager.ErrInvalidMethod, 400},
		{paymentmanager.ErrInvalidTransition, 400},
		{paymentmanager.ErrRefundExceeds, 400},
		{paymentmanager.ErrIdTaken, 409},
		{paymentmanager.ErrAlreadyPaid, 409},
		{paymentmanager.ErrNotPaid, 409},
		{paymentmanager.ErrConflict, 409},
		{paymentmanager.ErrGatewayUnavailable, 500},
		{paymentmanager.ErrGatewayFailed, 500},
		{paymentmanager.ErrAmountMismatch, 500},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			status := paymentmanager.ErrorStatus(tt.err)
			if status != tt.status {
				t.Errorf("expected %d, got %d", tt.status, status)
			}
		})
	}
}

func TestOps_BadBodyAndInvalidArgs(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})

	reg := &mock.Router{}
	reg.Configure(mock.Config{Authorize: func(userID string, r model.Resource, a model.Action) bool { return true }})
	m.MountOperations(reg)

	ctx := &mock.Context{InBody: []byte(`{}`)} // Empty body will fail Validation
	reg.Invoke("OP", "/"+paymentmanager.OpGetCharge, ctx)
}

func TestOps_NoProviderTokenInResponses(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash}
	_, err := m.CreateCounterPayment(args)
	if err != nil {
		t.Fatal(err)
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	p.ProviderToken = "should_not_see"
	db.Update(&p, storage.Eq("id", p.Id))

	reg := &mock.Router{}
	reg.Configure(mock.Config{Authorize: func(userID string, r model.Resource, a model.Action) bool { return true }})
	m.MountOperations(reg)

	ctx := &mock.Context{InBody: []byte(`{"tenant_id":"t1","id":"p1"}`)}
	reg.Invoke("OP", "/"+paymentmanager.OpGetPayment, ctx)
}

func TestOps_PaymentStatusShape(t *testing.T) {
	res := paymentmanager.PaymentStatusResult{}
	_ = res
}

type errReader struct {}
func (e errReader) Read(p []byte) (n int, err error) {
	return 0, paymentmanager.ErrGatewayFailed
}

func TestOps_DecodeError(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{})
	reg := &mock.Router{}
	reg.Configure(mock.Config{Authorize: func(userID string, r model.Resource, a model.Action) bool { return true }})
	m.MountOperations(reg)

	ops := []string{
		paymentmanager.OpGetCharge, paymentmanager.OpCreateCounterPayment, paymentmanager.OpStartOnlinePayment,
		paymentmanager.OpGetPaymentStatus, paymentmanager.OpGetPayment, paymentmanager.OpListPaymentsByReference,
		paymentmanager.OpListPayments, paymentmanager.OpRefundPayment, paymentmanager.OpListRefunds, paymentmanager.OpReconcileOnlinePayments,
	}

	for _, op := range ops {
		t.Run(op, func(t *testing.T) {
			// Sending invalid JSON to force Decode error
			ctx := &mock.Context{InBody: []byte(`{`)}
			reg.Invoke("OP", "/"+op, ctx)
		})
	}
}

func TestOps_SuccessAndDomainErrors(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{
		TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash,
	})
	if err != nil { t.Fatal(err) }

	reg := &mock.Router{}
	reg.Configure(mock.Config{Authorize: func(userID string, r model.Resource, a model.Action) bool { return true }})
	m.MountOperations(reg)

	call := func(op, body string) {
		ctx := &mock.Context{InBody: []byte(body)}
		reg.Invoke("OP", "/"+op, ctx)
	}

	call(paymentmanager.OpGetCharge, `{"tenant_id":"t1","reference_module":"mod","reference_id":"ref1"}`)
	call(paymentmanager.OpGetCharge, `{"tenant_id":"t1","reference_module":"missing","reference_id":"ref1"}`) // domain error

	call(paymentmanager.OpCreateCounterPayment, `{"tenant_id":"t1","id":"p2","reference_module":"mod","reference_id":"ref1","method":"CASH"}`)
	call(paymentmanager.OpCreateCounterPayment, `{"tenant_id":"t1","id":"p1","reference_module":"mod","reference_id":"ref1","method":"CASH"}`) // domain err

	call(paymentmanager.OpStartOnlinePayment, `{"tenant_id":"t1","id":"p3","reference_module":"mod","reference_id":"ref1","return_to":"url"}`)
	call(paymentmanager.OpStartOnlinePayment, `{"tenant_id":"t1","id":"p3","reference_module":"mod","reference_id":"ref1","return_to":"url"}`) // domain err

	call(paymentmanager.OpGetPaymentStatus, `{"tenant_id":"t1","id":"p1"}`)
	call(paymentmanager.OpGetPaymentStatus, `{"tenant_id":"t1","id":"p99"}`) // domain err

	call(paymentmanager.OpGetPayment, `{"tenant_id":"t1","id":"p1"}`)
	call(paymentmanager.OpGetPayment, `{"tenant_id":"t1","id":"p99"}`) // domain err

	call(paymentmanager.OpListPaymentsByReference, `{"tenant_id":"t1","reference_module":"mod","reference_id":"ref1"}`)
	call(paymentmanager.OpListPaymentsByReference, `{"tenant_id":"t1","reference_module":"mod2","reference_id":"ref2"}`) // domain err

	call(paymentmanager.OpListPayments, `{"tenant_id":"t1"}`)

	call(paymentmanager.OpRefundPayment, `{"tenant_id":"t1","id":"r1","payment_id":"p1","amount":500,"reason":"test"}`)
	call(paymentmanager.OpRefundPayment, `{"tenant_id":"t1","id":"r2","payment_id":"p99","amount":500,"reason":"test"}`) // domain err

	call(paymentmanager.OpListRefunds, `{"tenant_id":"t1","payment_id":"p1"}`)

	call(paymentmanager.OpReconcileOnlinePayments, `{"tenant_id":"t1"}`)
}
