with open("rule_payment_status_result_read.go", "r") as f:
    text = f.read()

text = text.replace('a.TenantID', 'filter.TenantId')
with open("rule_payment_status_result_read.go", "w") as f:
    f.write(text)

with open("rule_charge_result_read.go", "r") as f:
    text = f.read()

text = text.replace('a.TenantID', 'filter.TenantId')
with open("rule_charge_result_read.go", "w") as f:
    f.write(text)
