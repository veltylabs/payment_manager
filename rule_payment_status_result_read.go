package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/model"
)

func (p *PaymentStatusResult) Read(a crud.CurrentUser, filter *PaymentStatusResult, args crud.ReadArgs, m *Module) (model.ModelSlice, error) {
	if filter.Id == "" {
		return nil, ErrMissingArgs
	}
	pay, err := m.repo.paymentById(filter.TenantId, filter.Id)
	if err != nil {
		return nil, err
	}
	return &PaymentStatusResultList{
		&PaymentStatusResult{Id: pay.Id, Status: pay.Status},
	}, nil
}
