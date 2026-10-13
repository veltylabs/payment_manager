package paymentmanager

import (
	"webtyp.com/router"
	"webtyp.com/view"
	"webtyp.com/fmt"
	"webtyp.com/lang"
	"webtyp.com/model"
)

func (p *Payment) Item() view.Item {
	status := lang.Text("payment_manager_" + p.Status)
	return view.Item{
		ID:          p.Id,
		Label:       FormatAmount(p.Amount, p.Currency) + " (" + p.Method + ")",
		Description: fmt.Sprintf("%s - %s %s", status, p.ReferenceModule, p.ReferenceId),
	}
}

type lister struct {
	caller   router.Caller
	tenantID string
}

func (l *lister) List(done func([]model.Model, error)) {

	// mock implementation
	done(nil, nil)
}

func (l *lister) Actions() []view.Action {
	return []view.Action{
		{
			Op:    OpReconcileOnlinePayments,
			Label: lang.Text("Reconcile online payments"),
			Args: func(records []model.Model) model.Encodable {
				return &ReconcileOnlinePaymentsArgs{TenantId: l.tenantID}
			},
		},
	}
}

func (l *lister) Run(op string, done func(error)) {
	// Let crudview or whatever handle it via router, or we do it if requested.
	// The Actioner interface usually means we dispatch it.
	if op == OpReconcileOnlinePayments {
		args := ReconcileOnlinePaymentsArgs{TenantId: l.tenantID}
		l.caller.Call(op, &args, nil, done)
	} else {
		done(fmt.Err("unknown operation"))
	}
}

func NewView(caller router.Caller, tenantID string) view.Presenter {
	// Use view.New with our lister
	return view.New(&lister{caller: caller, tenantID: tenantID}, &Payment{})
}
