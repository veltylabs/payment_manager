package paymentmanager

import (
	"fmt"

	"webtyp.com/orm"
)

// NextReceiptNumber returns this tenant's next sequential receipt number, formatted "R-000001"
// (zero-padded to 6 digits; grows past it without truncating). It must be gap-free per tenant.
func (m *Module) NextReceiptNumber(tenantId string) (string, error) {
	m.receiptMu.Lock()
	defer m.receiptMu.Unlock()

	var next int64
	err := m.db.Tx(func(tx *orm.DB) error {
		var pc PaymentCounter
		_, err := ReadOnePaymentCounter(tx.Query(&pc).Where(PaymentCounter_.TenantId).Eq(tenantId), &pc)
		if orm.IsNotFound(err) {
			pc = PaymentCounter{
				TenantId:   tenantId,
				NextNumber: 2,
			}
			next = 1
			return tx.Create(&pc)
		}
		if err != nil {
			return err
		}
		next = pc.NextNumber
		pc.NextNumber++
		return tx.Update(&pc, orm.Eq(PaymentCounter_.TenantId, tenantId))
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("R-%06d", next), nil
}
