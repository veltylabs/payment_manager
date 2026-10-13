package paymentmanager

import (
	"webtyp.com/model"
	"webtyp.com/orm"
)

type repository struct {
	db *orm.DB
}

func (r *repository) paymentById(tenantId, id string) (Payment, error) {
	var p Payment
	err := r.db.Query(&p).Where("tenant_id").Eq(tenantId).Where("id").Eq(id).ReadOne()
	if err != nil {
		if orm.IsNotFound(err) {
			return p, ErrNotFound
		}
		return p, err
	}
	return p, nil
}

func (r *repository) paymentByProviderToken(token string) (Payment, error) {
	var p Payment
	err := r.db.Query(&p).Where("provider_token").Eq(token).ReadOne()
	if err != nil {
		if orm.IsNotFound(err) {
			return p, ErrNotFound
		}
		return p, err
	}
	return p, nil
}

func (r *repository) paymentsByReference(tenantId, refModule, refId string) ([]*Payment, error) {
	var list []*Payment
	err := r.db.Query(&Payment{}).Where("tenant_id").Eq(tenantId).Where("reference_module").Eq(refModule).Where("reference_id").Eq(refId).ReadAll(
		func() model.Model { return &Payment{} },
		func(m model.Model) { list = append(list, m.(*Payment)) },
	)
	return list, err
}

func (r *repository) paymentsByDateRange(tenantId string, from, to int64) ([]*Payment, error) {
	var list []*Payment
	err := r.db.Query(&Payment{}).Where("tenant_id").Eq(tenantId).Where("created_at").Gte(from).Where("created_at").Lt(to).ReadAll(
		func() model.Model { return &Payment{} },
		func(m model.Model) { list = append(list, m.(*Payment)) },
	)
	return list, err
}

func (r *repository) refundsByPayment(tenantId, paymentId string) ([]*Refund, error) {
	var list []*Refund
	err := r.db.Query(&Refund{}).Where("tenant_id").Eq(tenantId).Where("payment_id").Eq(paymentId).ReadAll(
		func() model.Model { return &Refund{} },
		func(m model.Model) { list = append(list, m.(*Refund)) },
	)
	return list, err
}

func (r *repository) refundById(tenantId, id string) (Refund, error) {
	var ref Refund
	err := r.db.Query(&ref).Where("tenant_id").Eq(tenantId).Where("id").Eq(id).ReadOne()
	if err != nil {
		if orm.IsNotFound(err) {
			return ref, ErrRefundNotFound
		}
		return ref, err
	}
	return ref, nil
}

func (r *repository) oldPendingOnlinePayments(tenantId string, before int64) ([]*Payment, error) {
	var list []*Payment
	err := r.db.Query(&Payment{}).Where("tenant_id").Eq(tenantId).Where("channel").Eq(ChannelOnline).Where("status").Eq(StatusPending).Where("created_at").Lt(before).ReadAll(
		func() model.Model { return &Payment{} },
		func(m model.Model) { list = append(list, m.(*Payment)) },
	)
	return list, err
}
