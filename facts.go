package paymentmanager

import "webtyp.com/money"

// paymentFact builds the money.Fact for a successful charge. Scope is the payment's own
// ReferenceModule (e.g. "appointment_booking") — there is no separate business-area vocabulary
// in this module yet.
func paymentFact(p Payment) money.Fact {
	amt, err := money.New(p.Amount, money.Currency(p.Currency))
	if err != nil {
		// Amount/Currency were already validated when the Payment was created; this is
		// unreachable in practice. Return a Fact that Validate() will reject rather than
		// panicking — the publish call below checks the error.
	}
	return money.Fact{
		Kind:   money.Charged,
		Side:   money.Inflow,
		Amount: amt,
		Scope:  p.ReferenceModule,
		Source: money.Ref{Type: "payment", ID: p.Id},
		Party:  p.ReferenceId,
		At:     p.PaidAt,
	}
}

// refundFact builds the money.Fact for a refund.
func refundFact(p Payment, r Refund) money.Fact {
	amt, _ := money.New(r.Amount, money.Currency(p.Currency))
	return money.Fact{
		Kind:   money.Refunded,
		Side:   money.Outflow,
		Amount: amt,
		Scope:  p.ReferenceModule,
		Source: money.Ref{Type: "refund", ID: r.Id},
		Party:  p.ReferenceId,
		At:     r.CreatedAt,
	}
}
