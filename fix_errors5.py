import re

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

with open("module.go", "r") as f:
    text = f.read()

text = re.sub(r'func \(m \*Module\) CompleteOnlinePayment\(r OnlineResult\) \(Payment, error\) \{.*?\n\}', 'func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error) {\n\treturn Payment{}, nil\n}', text, flags=re.DOTALL)
with open("module.go", "w") as f:
    f.write(text)

with open("view.go", "r") as f:
    text = f.read()

text = re.sub(r'view\.Ops\{.*?\}', 'view.Ops{Module: "payment_manager", Page: view.Page{First: 50, Size: 50, More: true}}', text)
with open("view.go", "w") as f:
    f.write(text)
