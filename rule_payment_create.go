package paymentmanager

import (
	"webtyp.com/crud"
	"webtyp.com/time"
)

func (p *Payment) Create(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error) {
	if p.Method == MethodCash || p.Method == MethodTransfer {
		// counter
		amount, currency, err := m.Due(p.TenantId, p.ReferenceModule, p.ReferenceId)
		if err != nil {
			return crud.RuleAnswer{}, err
		}
		p.Amount = amount
		p.Currency = currency
		p.Channel = ChannelCounter
		p.Status = StatusPaid
		p.PaidAt = time.NowUnix()
		p.CreatedBy = a.UserID
		return crud.ApplyAndPublish(), nil
	} else {
		// terminal
		if m.term == nil {
			return crud.RuleAnswer{}, ErrTerminalUnavailable
		}
		amount, currency, err := m.Due(p.TenantId, p.ReferenceModule, p.ReferenceId)
		if err != nil {
			return crud.RuleAnswer{}, err
		}
		if !m.hasTerminal(p.ReferenceModule) {
			return crud.RuleAnswer{}, ErrNoTerminalDevice
		}
		p.Amount = amount
		p.Currency = currency
		p.Channel = ChannelTerminal
		p.Status = StatusPending
		p.Provider = m.term.Provider()
		p.CreatedBy = a.UserID
		return crud.ApplyAndPublish(PaymentTerminalPush{}, PaymentTerminalStatus{}), nil
	}
}
