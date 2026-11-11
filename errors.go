package paymentmanager

import "webtyp.com/fmt"
import "webtyp.com/model"

var (
	ErrNotFound            = model.NotFound("payment not found")
	ErrRefundNotFound      = model.NotFound("refund not found")
	ErrMissingArgs         = model.Invalid("missing required arguments")
	ErrUnknownReference    = model.Invalid("no charge reader for this reference module")
	ErrNotPayable          = model.Invalid("the reference has nothing to pay")
	ErrUnsupportedCurrency = model.Invalid("unsupported currency")
	ErrInvalidMethod       = model.Invalid("invalid payment method for this channel")
	ErrInvalidTransition   = model.Invalid("invalid payment status transition")
	ErrRefundExceeds       = model.Invalid("refund amount exceeds what is left to refund")
	ErrIdTaken             = model.Conflict("id already used by another operation")
	ErrAlreadyPaid         = model.Conflict("the reference is already paid")
	ErrNotPaid             = model.Conflict("payment is not paid")
	ErrConflict            = model.Conflict("payment changed concurrently")
	ErrGatewayUnavailable  = model.Invalid("online payments are not configured")
	ErrGatewayFailed       = model.Invalid("payment gateway call failed")
	ErrAmountMismatch      = model.Invalid("gateway amount differs from the payment amount")
	ErrTerminalUnavailable = model.Invalid("card terminal payments are not configured")
	ErrNoTerminalDevice    = model.Invalid("no card terminal device for this reference module")
	ErrTerminalFailed      = model.Invalid("card terminal call failed")
	ErrTerminalRequired    = model.Invalid("card payments for this reference go through the card terminal")
	ErrNotTerminal         = model.Invalid("payment is not a card terminal payment")
	ErrTokenMismatch       = model.Invalid("the return's token is not this payment's; nothing was changed")
)

const ErrCannotRead = model.Invalid("payment_manager: cannot read the request")

var (
	ErrTimeoutOutOfRange   = fmt.Err("a wait is negative or longer than the module allows, so the module did not start; use a value between 1 and the limit documented in Timeouts")
	ErrTerminalUnreachable = fmt.Err("the card machine did not answer, so nothing was charged by this module; charge it on the machine and press «pago realizado correctamente»")
	ErrPaymentNotPending   = fmt.Err("this payment is no longer pending, so it cannot be confirmed")
	ErrStillPending        = fmt.Err("the machine has not finished yet; asking again shortly")
)
