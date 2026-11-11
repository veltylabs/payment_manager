package paymentmanager

import "webtyp.com/crud"

// StorePayment is the store of payment.
func (m *Module) StorePayment() crud.Store { return m.store }

// StoreRefund is the store of refund.
func (m *Module) StoreRefund() crud.Store { return m.store }

// StoreOnlinePaymentRequest is the store of payment_online_request.
func (m *Module) StoreOnlinePaymentRequest() crud.Store { return m.store }

// StorePaymentManualConfirmation is the store of payment_manual_confirmation.
func (m *Module) StorePaymentManualConfirmation() crud.Store { return m.store }

// StorePaymentReconciliation is the store of payment_reconciliation.
func (m *Module) StorePaymentReconciliation() crud.Store { return m.store }
