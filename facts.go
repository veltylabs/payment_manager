package paymentmanager

import "webtyp.com/money"

// Origen de los hechos.
const (
	SourcePayment    = "payment"
	SourcePaymentFee = "payment_fee"
	SourceRefund     = "refund"
)

// meansOf traduce el medio del pago al medio del hecho.
func meansOf(method string) money.Means {
	switch method {
	case MethodCash:
		return money.MeansCash
	case MethodTransfer:
		return money.MeansTransfer
	}
	return money.MeansCard // debit, credit, card
}

// paymentFacts arma los hechos de un pago recién pagado: Charged si el dinero está en mano; con tarjeta, lo que
// debe el procesador (Accrued de entrada, vence el día del depósito) y su comisión (Accrued de salida).
func (m *Module) paymentFacts(p Payment) ([]money.Fact, error) {
	amt, err := money.New(p.Amount, money.Currency(p.Currency))
	if err != nil {
		return nil, err
	}
	means := meansOf(p.Method)
	base := money.Fact{Amount: amt, Scope: p.ReferenceModule, Source: money.Ref{Type: SourcePayment, ID: p.Id}, At: p.PaidAt, Means: means, Actor: p.CreatedBy}
	if means != money.MeansCard {
		base.Kind, base.Side, base.Party = money.Charged, money.Inflow, p.ReferenceId
		return []money.Fact{base}, nil
	}
	base.Kind, base.Side, base.Party = money.Accrued, money.Inflow, p.Provider
	terms, ok := m.payoutTerms(p.Channel)
	if !ok {
		return []money.Fact{base}, nil
	}
	po, err := terms.Payout(PayoutCharge{Amount: p.Amount, Currency: p.Currency, Method: p.Method, Channel: p.Channel})
	if err != nil {
		return nil, err
	}
	due, err := m.bank.BankDayAfter(p.PaidAt, po.BusinessDays)
	if err != nil {
		return nil, err
	}
	base.DueAt = due
	facts := []money.Fact{base}
	if cost := po.Fee + po.FeeTax; cost > 0 {
		feeAmt, err := money.New(cost, money.Currency(p.Currency))
		if err != nil {
			return nil, err
		}
		facts = append(facts, money.Fact{Kind: money.Accrued, Side: money.Outflow, Amount: feeAmt, Scope: p.ReferenceModule,
			Source: money.Ref{Type: SourcePaymentFee, ID: p.Id}, Party: p.Provider, At: p.PaidAt, DueAt: due, Means: money.MeansCard, Actor: p.CreatedBy})
	}
	return facts, nil
}

// payoutTerms devuelve el PayoutTerms del adaptador del canal, si lo implementa.
func (m *Module) payoutTerms(channel string) (PayoutTerms, bool) {
	var a any
	switch channel {
	case ChannelTerminal:
		a = m.term
	case ChannelOnline:
		a = m.gw
	}
	t, ok := a.(PayoutTerms)
	return t, ok
}

// refundFact arma el hecho de una devolución, con el medio del pago devuelto.
func refundFact(p Payment, r Refund) (money.Fact, error) {
	amt, err := money.New(r.Amount, money.Currency(p.Currency))
	if err != nil {
		return money.Fact{}, err
	}
	return money.Fact{Kind: money.Refunded, Side: money.Outflow, Amount: amt, Scope: p.ReferenceModule,
		Source: money.Ref{Type: SourceRefund, ID: r.Id}, Party: p.ReferenceId, At: r.CreatedAt, Means: meansOf(p.Method), Actor: r.CreatedBy}, nil
}
