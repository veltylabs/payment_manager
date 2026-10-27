package tests

import (
	"testing"

	"webtyp.com/fmt"

	"webtyp.com/model"
	"webtyp.com/router/loopback"

	pm "github.com/veltylabs/payment_manager"
)

// errCaller responde todo con un error de transporte.
type errCaller struct{}

func (errCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	done(errSimulated)
}
func (errCaller) Dispatch(op string, args model.Encodable) {}

func TestView_ListsRecentPayments(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	presenter := pm.NewView(loopback.WithTenant(tenant, e.m))
	var got error = errSimulated
	presenter.Reload(func(err error) { got = err })
	if got != nil {
		t.Fatalf("Reload: %v", got)
	}
	items := presenter.Items()
	if len(items) != 1 || items[0].ID != "p1" {
		t.Fatalf("items = %+v", items)
	}
	if !fmt.Contains(items[0].Label, "$25.000") || !fmt.Contains(items[0].Description, refID) {
		t.Fatalf("item = %+v", items[0])
	}
	if presenter.Title() != "Payments" {
		t.Fatalf("title = %q", presenter.Title())
	}
}

func TestView_ListErrorPropagates(t *testing.T) {
	var got error
	pm.NewView(errCaller{}).Reload(func(err error) { got = err })
	if got != errSimulated {
		t.Fatalf("err = %v", got)
	}
}
