import re

with open("definition.go", "r") as f:
    text = f.read()

# Replace missing `)` in regex
to_delete = [
    "CreateCounterPaymentArgsModel",
    "StartOnlinePaymentArgsModel",
    "StartOnlinePaymentResultModel",
    "GetPaymentArgsModel",
    "GetChargeArgsModel",
    "StartTerminalPaymentArgsModel",
    "ListPaymentsByReferenceArgsModel",
    "ListPaymentsArgsModel",
    "RefundPaymentArgsModel",
    "ListRefundsArgsModel",
    "ReconcilePaymentsArgsModel",
    "ReconcileResultModel"
]

for model in to_delete:
    pattern = r'var ' + model + r' = model\.Definition\{.*?\n\}\n'
    text = re.sub(pattern, '', text, flags=re.DOTALL)

new_models = """
var OnlinePaymentRequestModel = model.Definition{
	Name: "payment_online_request",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		{Name: "return_url", Type: input.Text()},
		{Name: "payment_id", Type: model.Text(), ServerOnly: true},
		{Name: "redirect_url", Type: model.Text(), ServerOnly: true},
		{Name: "status", Type: model.Text(), ServerOnly: true},
		{Name: "failure", Type: model.Text(), ServerOnly: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
		{Name: "payer_name", Type: model.Text()},
		{Name: "payer_email", Type: model.Text()},
		{Name: "payer_phone", Type: model.Text()},
		{Name: "token", Type: model.Text(), ServerOnly: true},
		{Name: "token_field", Type: model.Text(), ServerOnly: true},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "GatewayStart", Mode: model.WaitForReply, MaxWaitSeconds: 15},
	},
}

var PaymentReconciliationModel = model.Definition{
	Name: "payment_reconciliation",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "status", Type: model.Text(), ServerOnly: true},
		{Name: "failure", Type: model.Text(), ServerOnly: true},
		{Name: "reconciled", Type: model.Int(), ServerOnly: true},
		{Name: "actor_id", Type: model.Text(), ServerOnly: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "Reconcile", Mode: model.RetryInBackground, MaxAttempts: 3},
	},
}

var PaymentManualConfirmationModel = model.Definition{
	Name: "payment_manual_confirmation",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "payment_id", Type: model.Text(), NotNull: true},
		{Name: "note", Type: input.Text(), NotNull: true},
		{Name: "actor_id", Type: model.Text(), ServerOnly: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "Confirm", Mode: model.WaitForReply, MaxWaitSeconds: 10},
	},
}
"""

text += new_models

# Add Allow, Rules and ExternalCalls to PaymentModel and RefundModel
text = re.sub(r'(var PaymentModel = model\.Definition\{\n.*?\n)\}', r'\1\tAllow: model.Create | model.Read,\n\tRules: model.Rules{\n\t\tCreate: model.AddMyBusinessRule,\n\t\tRead: model.HaveNoBusinessRule,\n\t},\n\tExternalCalls: []model.ExternalCallDefinition{\n\t\t{Name: "TerminalPush", Mode: model.WaitForReply, MaxWaitSeconds: 10},\n\t\t{Name: "TerminalStatus", Mode: model.RetryInBackground, MaxAttempts: 20},\n\t},\n}', text, flags=re.DOTALL)

text = re.sub(r'(var RefundModel = model\.Definition\{\n.*?\n)\}', r'\1\tAllow: model.Create | model.Read,\n\tRules: model.Rules{\n\t\tCreate: model.AddMyBusinessRule,\n\t\tRead: model.HaveNoBusinessRule,\n\t},\n\tExternalCalls: []model.ExternalCallDefinition{\n\t\t{Name: "GatewayRefund", Mode: model.WaitForReply, MaxWaitSeconds: 15},\n\t},\n}', text, flags=re.DOTALL)

text = re.sub(r'(var ChargeResultModel = model\.Definition\{\n.*?\n)\}', r'\1\tAllow: model.Read,\n\tRules: model.Rules{\n\t\tRead: model.AddMyBusinessRule,\n\t},\n}', text, flags=re.DOTALL)

text = re.sub(r'(var PaymentStatusResultModel = model\.Definition\{\n.*?\n)\}', r'\1\tAllow: model.Read,\n\tRules: model.Rules{\n\t\tRead: model.AddMyBusinessRule,\n\t},\n}', text, flags=re.DOTALL)


# Create Definition at the end
text += """

var Definition = model.ModuleDefinition{
	Name: "payment_manager",
	Entities: []model.Definition{
		PaymentModel,
		PaymentCounterModel,
		PaymentManualConfirmationModel,
		OnlinePaymentRequestModel,
		RefundModel,
		PaymentReconciliationModel,
		ChargeResultModel,
		PaymentStatusResultModel,
	},
}
"""

with open("definition.go", "w") as f:
    f.write(text)
