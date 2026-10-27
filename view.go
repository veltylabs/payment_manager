package paymentmanager

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
)

// ListWindowSeconds es cuánto hacia atrás muestra la lista de pagos: 30 días.
const ListWindowSeconds = 30 * time.Day

const titlePayments = "Payments"

// NewView construye el Presenter de solo lectura de los pagos de los últimos ListWindowSeconds.
//
// Uso:
//  presenter := paymentmanager.NewView(caller, view.WithTitle("Pagos"))
func NewView(caller router.Caller, opts ...view.Option) view.Presenter {
	allOpts := append([]view.Option{view.WithTitle(titlePayments)}, opts...)
	lister := view.NewCallerLister(view.NewRecords().Caller(caller), view.Ops{Module: ModelName, List: OpListPayments}, func() model.ModelSlice { return &PaymentList{} })
	return view.New(lister, &Payment{}, allOpts...)
}
