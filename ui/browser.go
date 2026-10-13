package ui

import (
	paymentmanager "github.com/veltylabs/payment_manager"
	. "webtyp.com/dom"
	"webtyp.com/layout/crudview"
	"webtyp.com/layout/platformd"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/svg"
)

func Browser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error) {
	return platformd.NewUIModule(ID, ModLabel, svg.Icon(ID), newPaymentsView(caller, ids, tenantID)), nil
}

type paymentsView struct {
	Element
	caller   router.Caller
	ids      model.IDGenerator
	tenantID string
}

func newPaymentsView(caller router.Caller, ids model.IDGenerator, tenantID string) *paymentsView {
	view := &paymentsView{
		caller:   caller,
		ids:      ids,
		tenantID: tenantID,
	}

	viewPresenter := paymentmanager.NewView(caller, tenantID)
	cv, _ := crudview.New(crudview.Config{
		ParentID:  ID,
		Presenter: viewPresenter,
		IDs:       ids,
	})
	view.Element = *cv.Render()
	return view
}

func (v *paymentsView) Render() *Element {
	return &v.Element
}
