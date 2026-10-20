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

// NewView construye el Presenter de solo lectura de los pagos del tenant de
// los últimos ListWindowSeconds.
func NewView(caller router.Caller, tenantId string) view.Presenter {
	return view.New(&paymentLister{caller: caller, tenantId: tenantId}, &Payment{}, view.WithTitle(titlePayments))
}

type paymentLister struct {
	caller   router.Caller
	tenantId string
}

func (l *paymentLister) List(done func([]model.Model, error)) {
	now := time.NowUnix()
	out := &PaymentList{}
	l.caller.Call(ModelName+"."+OpListPayments,
		&ListPaymentsArgs{TenantId: l.tenantId, From: int64(now - ListWindowSeconds), To: int64(now + 1)}, out,
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
