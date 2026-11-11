//go:build wasm

package seed

import (
	paymentmanager "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/migrate"
	"github.com/veltylabs/payment_manager/ui"
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/orm"
	"webtyp.com/storage/mem"
	"webtyp.com/unixid"
	"webtyp.com/view"
)

const (
	demoTenantID    = "demo"
	demoRefModule   = "demo_booking"
	demoCurrency    = "CLP"
	demoChargeCents = 25000
)

// demoCharge es el módulo dueño de la referencia: siempre hay 25.000 por cobrar.
type demoCharge struct{}

func (demoCharge) Charge(tenantId, referenceId string) (int64, string, bool, error) {
	return demoChargeCents, demoCurrency, true, nil
}

// Demo crea una demostración ejecutable del módulo payment_manager sobre una base en memoria con pagos de ejemplo.
//
// Ejemplo: demo, err := seed.Demo()
func Demo() (view.Demo, error) {
	db := orm.New(mem.New())
	ids, err := unixid.NewUnixID()
	if err != nil {
		return view.Demo{}, err
	}
	mod, err := paymentmanager.New(db, paymentmanager.Deps{
		IDs:      ids,
		TenantID: demoTenantID,
		Charges:  []paymentmanager.ChargeSource{{Module: demoRefModule, Reader: demoCharge{}}},
	})
	if err != nil {
		return view.Demo{}, err
	}
	if _, err := Load(mod, demoTenantID, []Ref{{Module: demoRefModule, Id: "r1"}, {Module: demoRefModule, Id: "r2"}}); err != nil {
		return view.Demo{}, err
	}
	return view.Demo{
		Label:      string(ui.NavLabel),
		Timezone:   "America/Santiago",
		ActingAs:   "u1",
		Modules:    []crud.Module{mod},
		Migrations: []ddl.Step{{Name: paymentmanager.Definition.Name, Run: migrate.Migrate}},
		Screens: func(d view.BrowserDeps) []view.Screen {
			return ui.Screens(d.Caller)
		},
	}, nil
}
