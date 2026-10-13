package seed

import (
	paymentmanager "github.com/veltylabs/payment_manager"
)

type Ref struct {
	Module string
	Id     string
}

type Data struct {
	Payments []paymentmanager.Payment
	Refunds  []paymentmanager.Refund
}

func Load(m *paymentmanager.Module, tenantID string, refs []Ref) (Data, error) {
	var data Data
	if len(refs) == 0 {
		return data, nil
	}

	p1, err := m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{
		TenantId:        tenantID,
		Id:              "seed_p1",
		ReferenceModule: refs[0].Module,
		ReferenceId:     refs[0].Id,
		Method:          paymentmanager.MethodCash,
	})
	if err != nil {
		return data, err
	}
	data.Payments = append(data.Payments, p1)

	if len(refs) > 1 {
		p2, err := m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{
			TenantId:        tenantID,
			Id:              "seed_p2",
			ReferenceModule: refs[1].Module,
			ReferenceId:     refs[1].Id,
			Method:          paymentmanager.MethodDebit,
		})
		if err != nil {
			return data, err
		}
		data.Payments = append(data.Payments, p2)

		r1, err := m.RefundPayment(paymentmanager.RefundPaymentArgs{
			TenantId:  tenantID,
			Id:        "seed_r1",
			PaymentId: p2.Id,
			Amount:    p2.Amount / 2,
			Reason:    "Seed refund",
			Method:    paymentmanager.MethodDebit,
		})
		if err != nil {
			return data, err
		}
		data.Refunds = append(data.Refunds, r1)
	}

	return data, nil
}
