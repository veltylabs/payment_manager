import os

with open("definition.go", "r") as f:
    lines = f.readlines()

out = []
in_def = False
skip_names = [
    "CreateCounterPaymentArgsModel", "StartOnlinePaymentArgsModel",
    "StartOnlinePaymentResultModel", "GetPaymentArgsModel",
    "GetChargeArgsModel", "StartTerminalPaymentArgsModel",
    "ListPaymentsByReferenceArgsModel", "ListPaymentsArgsModel",
    "RefundPaymentArgsModel", "ListRefundsArgsModel",
    "ReconcilePaymentsArgsModel", "ReconcileResultModel"
]

to_keep = []

# But wait, looking at PLAN_STAGE_1:
# Delete every *Args / *Result definition that only carried a removed operation
# Keep every *Args model that an exported reader's signature still uses (grep -n "^func (m \*Module) [A-Z][A-Za-z]*([^)]*Args" *.go lists them): mark each Wire: true; delete only the *Args no remaining signature uses.
# The remaining args are: CreateCounterPaymentArgs, StartOnlinePaymentArgs, StartTerminalPaymentArgs, RefundPaymentArgs?
# Wait! "CreateCounterPayment", "StartOnlinePayment", "StartTerminalPayment", "RefreshTerminalPayment" and "RefundPayment" are DELETED in Stage 4.
# Ah, Stage 4 says:
# Keep Charge, Due, PaidAmount, CompleteOnlinePayment, and ReconcilePayments.
# Delete CreateCounterPayment, StartOnlinePayment, StartTerminalPayment, RefreshTerminalPayment, RefundPayment.
# So if those are deleted, their Args are not used by any exported reader's signature!

# Wait, Stage 1 says: "Keep every *Args model that an exported reader's signature still uses... mark each Wire: true; delete only the *Args no remaining signature uses."
# If Charge uses GetChargeArgs? Wait, Charge is func (m *Module) Charge(tenantId, referenceModule, referenceId string) - it doesn't take Args!
