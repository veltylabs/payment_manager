package seed

import (
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/router"
)

type Ref struct {
	Module string
	Id     string
}

type Data struct {
	Payments []paymentmanager.Payment
	Refunds  []paymentmanager.Refund
}

// Load crea pagos y devoluciones de ejemplo por las mismas operaciones que usa la app (el cargo se calcula por la regla, nunca se
// escribe a mano). refs son las referencias (módulo e id) sobre las que se cobra.
//
// Ejemplo: data, err := seed.Load(mod, "t1", []seed.Ref{{Module: "booking", Id: "r1"}})
func Load(m *paymentmanager.Module, tenantID string, refs []Ref) (Data, error) {
	var data Data
	if len(refs) == 0 {
		return data, nil
	}
	caller := crud.LoopbackActingAs(nil, "seed", m)

	pay := func(id string, ref Ref, method string) (paymentmanager.Payment, error) {
		var out paymentmanager.Payment
		var callErr error
		caller.Call(router.Operation{Module: paymentmanager.ModelName, Entity: (&paymentmanager.Payment{}).ModelName(), Action: model.Create},
			&crud.CreateArgs{Records: []model.Model{&paymentmanager.Payment{Id: id, ReferenceModule: ref.Module, ReferenceId: ref.Id, Method: method}}},
			&out, func(err error) { callErr = err })
		return out, callErr
	}

	p1, err := pay("seed_p1", refs[0], paymentmanager.MethodCash)
	if err != nil {
		return data, err
	}
	data.Payments = append(data.Payments, p1)

	if len(refs) > 1 {
		p2, err := pay("seed_p2", refs[1], paymentmanager.MethodTransfer)
		if err != nil {
			return data, err
		}
		data.Payments = append(data.Payments, p2)

		var r1 paymentmanager.Refund
		var callErr error
		caller.Call(router.Operation{Module: paymentmanager.ModelName, Entity: (&paymentmanager.Refund{}).ModelName(), Action: model.Create},
			&crud.CreateArgs{Records: []model.Model{&paymentmanager.Refund{Id: "seed_r1", PaymentId: p2.Id, Amount: p2.Amount / 2, Reason: "Seed refund", Method: paymentmanager.MethodTransfer}}},
			&r1, func(err error) { callErr = err })
		if callErr != nil {
			return data, callErr
		}
		data.Refunds = append(data.Refunds, r1)
	}
	return data, nil
}
