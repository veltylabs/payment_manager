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

func NewView(caller router.Caller, opts ...view.Option) view.Presenter {
	allOpts := append([]view.Option{view.WithTitle(titlePayments)}, opts...)
	lister := view.NewCallerLister(caller, view.Ops{Module: "payment_manager", Page: view.Page{First: 50, Size: 50, More: true}}, func() model.TableSlice { return &PaymentList{} })
	return view.New(lister, &Payment{}, allOpts...)
}
