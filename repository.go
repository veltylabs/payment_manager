package paymentmanager

import (
	"webtyp.com/model"
	"webtyp.com/orm"
)

// repository reúne las consultas del módulo. Toda lectura acotada por tenant
// filtra por tenant_id; toda escritura lo lleva en la condición.
type repository struct {
	db *orm.DB
}

// readOnePayment traduce "no encontrado" al centinela de dominio; cualquier otro
// error de la base se propaga tal cual.
// p debe ser el mismo modelo que se pasó a Query: ReadOne escanea en él.
func readOnePayment(qb *orm.QB, p *Payment) (Payment, error) {
	got, err := ReadOnePayment(qb, p)
	if orm.IsNotFound(err) {
		return Payment{}, ErrNotFound
	}
	if err != nil {
		return Payment{}, err
	}
	return *got, nil
}

func (r *repository) paymentById(tenantId, id string) (Payment, error) {
	p := &Payment{}
	return readOnePayment(r.db.Query(p).Where(Payment_.TenantId).Eq(tenantId).Where(Payment_.Id).Eq(id), p)
}

// paymentAnyTenant busca un id sin filtrar por tenant: los ids los genera
// quien llama, así que reusarlos entre tenants debe responder ErrIdTaken.
func (r *repository) paymentAnyTenant(id string) (Payment, error) {
	p := &Payment{}
	return readOnePayment(r.db.Query(p).Where(Payment_.Id).Eq(id), p)
}

func (r *repository) paymentByProviderToken(token string) (Payment, error) {
	p := &Payment{}
	return readOnePayment(r.db.Query(p).Where(Payment_.ProviderToken).Eq(token), p)
}

func (r *repository) paymentsByReference(tenantId, refModule, refId string) ([]Payment, error) {
	return readAllPayments(r.db.Query(&Payment{}).
		Where(Payment_.TenantId).Eq(tenantId).
		Where(Payment_.ReferenceModule).Eq(refModule).
		Where(Payment_.ReferenceId).Eq(refId))
}

func (r *repository) paymentsByDateRange(tenantId string, from, to int64) ([]Payment, error) {
	return readAllPayments(r.db.Query(&Payment{}).
		Where(Payment_.TenantId).Eq(tenantId).
		Where(Payment_.CreatedAt).Gte(from).
		Where(Payment_.CreatedAt).Lt(to))
}

func (r *repository) oldPendingOnlinePayments(tenantId string, before int64) ([]Payment, error) {
	return readAllPayments(r.db.Query(&Payment{}).
		Where(Payment_.TenantId).Eq(tenantId).
		Where(Payment_.Channel).Eq(ChannelOnline).
		Where(Payment_.Status).Eq(StatusPending).
		Where(Payment_.CreatedAt).Lt(before))
}

func readAllPayments(qb *orm.QB) ([]Payment, error) {
	rows, err := ReadAllPayment(qb)
	if err != nil {
		return nil, err
	}
	out := make([]Payment, len(rows))
	for i, row := range rows {
		out[i] = *row
	}
	return out, nil
}

func (r *repository) refundAnyTenant(id string) (Refund, error) {
	ref := &Refund{}
	got, err := ReadOneRefund(r.db.Query(ref).Where(Refund_.Id).Eq(id), ref)
	if orm.IsNotFound(err) {
		return Refund{}, ErrRefundNotFound
	}
	if err != nil {
		return Refund{}, err
	}
	return *got, nil
}

func (r *repository) refundsByPayment(tenantId, paymentId string) ([]Refund, error) {
	rows, err := ReadAllRefund(r.db.Query(&Refund{}).Where(Refund_.TenantId).Eq(tenantId).Where(Refund_.PaymentId).Eq(paymentId))
	if err != nil {
		return nil, err
	}
	out := make([]Refund, len(rows))
	for i, row := range rows {
		out[i] = *row
	}
	return out, nil
}

// updatePaymentTx guarda p dentro de tx solo si su revisión guardada sigue
// siendo expected (concurrencia optimista); si no, ErrConflict.
func (r *repository) updatePaymentTx(tx *orm.DB, p *Payment, expected int64) error {
	got := &Payment{}
	current, err := readOnePayment(tx.Query(got).Where(Payment_.TenantId).Eq(p.TenantId).Where(Payment_.Id).Eq(p.Id), got)
	if err != nil {
		return err
	}
	if current.Revision != expected {
		return ErrConflict
	}
	if err := p.Validate(byte(model.ActionUpdate)); err != nil {
		return err
	}
	return tx.Update(p, orm.Eq(Payment_.Id, p.Id), orm.Eq(Payment_.TenantId, p.TenantId))
}
