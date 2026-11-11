package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/model"
)

func (p *ChargeResult) Read(a crud.CurrentUser, filter *ChargeResult, args crud.ReadArgs, m *Module) (model.ModelSlice, error) {
	if filter.ReferenceModule == "" || filter.ReferenceId == "" {
		return nil, ErrMissingArgs
	}
	amount, currency, payable, err := m.Charge(filter.TenantId, filter.ReferenceModule, filter.ReferenceId)
	if err != nil {
		return nil, err
	}
	terminal := m.hasTerminal(filter.ReferenceModule)
	return &ChargeResultList{
		&ChargeResult{
			Amount: amount,
			Currency: currency,
			Payable: payable,
			Terminal: terminal,
		},
	}, nil
}
