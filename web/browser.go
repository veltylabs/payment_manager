//go:build browser

package main

import (
	. "webtyp.com/dom"
	"webtyp.com/events/mock"
	"webtyp.com/layout/platformd"
	"webtyp.com/orm"
	"webtyp.com/router/loopback"
	"webtyp.com/storage/mem"
	"webtyp.com/svg"
	"webtyp.com/unixid"

	paymentmanager "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/sandbox"
	"github.com/veltylabs/payment_manager/seed"
	"github.com/veltylabs/payment_manager/ui"
)

// demoTenantID es el único tenant de esta demo.
const demoTenantID = "demo"

// demoModule es el módulo de referencia de la demo: todo lo que se cobra vale
// $25.000 CLP.
const demoModule = "demo"

type demoCharges struct{}

func (demoCharges) Charge(tenantId, referenceId string) (int64, string, bool, error) {
	return 25000, paymentmanager.CurrencyCLP, true, nil
}

type demoUser struct{}

func (demoUser) UserName() string    { return "Demo" }
func (demoUser) UserAvatar() string  { return "" }
func (demoUser) UserRoles() []string { return []string{"Administrador"} }

func main() {
	ids, err := unixid.NewUnixID()
	if err != nil {
		panic(err)
	}
	pm, err := paymentmanager.New(orm.New(mem.New()), paymentmanager.Deps{
		Publisher: &mock.Broker{},
		Gateway:   sandbox.New("/"),
		Charges:   []paymentmanager.ChargeSource{{Module: demoModule, Reader: demoCharges{}}},
	})
	if err != nil {
		panic(err)
	}
	if _, err := seed.Load(pm, demoTenantID, []seed.Ref{{Module: demoModule, Id: "visita-1"}, {Module: demoModule, Id: "visita-2"}}); err != nil {
		panic(err)
	}

	caller := loopback.WithTenant(demoTenantID, pm)
	payments, err := ui.Browser(caller, ids, demoTenantID)
	if err != nil {
		panic(err)
	}
	step := &ui.PaymentStep{
		Caller:          caller,
		IDs:             ids,
		TenantID:        demoTenantID,
		Channel:         paymentmanager.ChannelCounter,
		ReferenceModule: demoModule,
		ReferenceID:     "visita-3",
	}
	cashier := platformd.NewUIModule("payment_step_demo", "Cobrar (demo)", svg.Icon(ui.ID), step)

	Append("body", &platformd.Platform{
		AppName:   "Pagos — demo",
		User:      demoUser{},
		Modules:   []platformd.UIModule{payments, cashier},
		DefaultID: ui.ID,
	})
	select {}
}
