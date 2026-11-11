package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/time"
)

// call_payment_terminal_status.go — RetryInBackground: consulta hasta que la máquina responda o termine el período
func (p *Payment) TerminalStatus(a crud.CurrentUser, m *Module, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
	if p.Status != StatusPending {
		return nil, nil
	}
	if int64(time.NowUnix()-p.CreatedAt) > m.timeouts.TerminalPayWindowSeconds {
		return nil, nil // período terminado: se mantiene pendiente y se ofrece confirmación manual
	}
	if p.ProviderToken == "" {
		return nil, ErrTerminalUnreachable
	}
	res, err := m.term.Status(p.ProviderToken)
	if err != nil {
		return nil, ErrTerminalUnreachable
	}
	settled, err := m.settleTerminal(*p, res, false)
	if err != nil {
		return nil, err
	}
	if settled.Status == StatusPending {
		return nil, ErrStillPending
	}
	return nil, nil
}
