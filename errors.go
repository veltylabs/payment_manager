package paymentmanager

type domainError string

func (e domainError) Error() string { return string(e) }

const (
	ErrNotFound            = domainError("payment not found")
	ErrRefundNotFound      = domainError("refund not found")
	ErrMissingArgs         = domainError("missing required arguments")
	ErrUnknownReference    = domainError("no charge reader for this reference module")
	ErrNotPayable          = domainError("the reference has nothing to pay")
	ErrUnsupportedCurrency = domainError("unsupported currency")
	ErrInvalidMethod       = domainError("invalid payment method for this channel")
	ErrInvalidTransition   = domainError("invalid payment status transition")
	ErrRefundExceeds       = domainError("refund amount exceeds what is left to refund")
	ErrIdTaken             = domainError("id already used by another operation")
	ErrAlreadyPaid         = domainError("the reference is already paid")
	ErrNotPaid             = domainError("payment is not paid")
	ErrConflict            = domainError("payment changed concurrently")
	ErrGatewayUnavailable  = domainError("online payments are not configured")
	ErrGatewayFailed       = domainError("payment gateway call failed")
	ErrAmountMismatch      = domainError("gateway amount differs from the payment amount")
)
