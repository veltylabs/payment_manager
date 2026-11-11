import re

with open("definition.go", "r") as f:
    text = f.read()

# Add fields to ChargeResultModel
text = re.sub(
    r'(var ChargeResultModel = model\.Definition\{\n\tName: "charge_result",\n\tFields: model\.Fields\{\n)',
    r'\1\t\t{Name: "tenant_id", Type: model.Text(), NotNull: true},\n\t\t{Name: "reference_module", Type: model.Text(), NotNull: true},\n\t\t{Name: "reference_id", Type: model.Text(), NotNull: true},\n',
    text
)

# Add fields to PaymentStatusResultModel
text = re.sub(
    r'(var PaymentStatusResultModel = model\.Definition\{\n\tName: "payment_status_result",\n\tFields: model\.Fields\{\n\t\t{Name: "id", Type: model.Text()},\n)',
    r'\1\t\t{Name: "tenant_id", Type: model.Text(), NotNull: true},\n',
    text
)

with open("definition.go", "w") as f:
    f.write(text)
