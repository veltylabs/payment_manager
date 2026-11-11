with open("rule_payment_status_result_read.go", "r") as f:
    text = f.read()

text = text.replace('args.TenantID', 'a.TenantID')
with open("rule_payment_status_result_read.go", "w") as f:
    f.write(text)

with open("rule_charge_result_read.go", "r") as f:
    text = f.read()

text = text.replace('args.TenantID', 'a.TenantID')
with open("rule_charge_result_read.go", "w") as f:
    f.write(text)
