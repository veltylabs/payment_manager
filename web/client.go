//go:build wasm

package main

import (
	"webtyp.com/dom"
	"webtyp.com/storage/mem"
	"webtyp.com/orm"
	"webtyp.com/events/mock"
	"webtyp.com/router/loopback"
	"webtyp.com/unixid"
	paymentmanager "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/sandbox"
	"github.com/veltylabs/payment_manager/ui"
	"github.com/veltylabs/payment_manager/seed"
)

type demoCharges struct{}

func (demoCharges) Charge(tenantId, referenceId string) (int64, string, bool, error) {
	return 25000, paymentmanager.CurrencyCLP, true, nil
}

func main() {
	db := orm.New(mem.New())
	pub := mock.NewPublisher()
	gw := sandbox.New("/")

	m, err := paymentmanager.New(db, paymentmanager.Deps{
		Publisher: pub,
		Gateway:   gw,
		ReturnURL: "/",
		Charges: []paymentmanager.ChargeSource{
			{Module: "demo", Reader: demoCharges{}},
		},
	})
	if err != nil {
		panic(err)
	}

	tenantID := "demo-tenant"
	_, err = seed.Load(m, tenantID, []seed.Ref{
		{Module: "demo", Id: "ref1"},
		{Module: "demo", Id: "ref2"},
	})
	if err != nil {
		panic(err)
	}

	c := loopback.NewCaller(m)
	ids := unixid.NewUnixID()

	browser, err := ui.Browser(c, ids, tenantID)
	if err != nil {
		panic(err)
	}

	dom.MountBody(browser)
}
