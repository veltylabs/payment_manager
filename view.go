package paymentmanager

import (
	"webtyp.com/lang"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
)

// ListWindowSeconds es cuánto hacia atrás muestra la lista de pagos: 30 días.
const ListWindowSeconds = 30 * time.Day

const titlePayments = "Payments"

// Item proyecta un pago como fila: monto y medio; estado y referencia.
func (p *Payment) Item() view.Item {
	return view.Item{
		ID:          p.Id,
		Label:       FormatAmount(p.Amount, p.Currency) + " · " + lang.Translate(p.Method).String(),
		Description: lang.Translate(p.Status).String() + " · " + p.ReferenceModule + " " + p.ReferenceId,
	}
}

// NewView construye el Presenter de solo lectura de los pagos de los últimos ListWindowSeconds.
func NewView(caller router.Caller, opts ...view.Option) view.Presenter {
	allOpts := append([]view.Option{view.WithTitle(titlePayments)}, opts...)
	return view.New(&paymentLister{caller: caller}, &Payment{}, allOpts...)
}

type paymentLister struct {
	caller router.Caller
}

func (l *paymentLister) List(done func([]model.Model, error)) {
	out := &PaymentList{}
	l.caller.Call(ModelName+"."+OpListPayments,
		nil, out,
		func(err error) {
			if err != nil {
				done(nil, err)
				return
			}
			rows := make([]model.Model, len(*out))
			for i, p := range *out {
				rows[i] = p
			}
			done(rows, nil)
		})
}
