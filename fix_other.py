import re
with open("rule_charge_result_read.go", "r") as f:
    text = f.read()
text = text.replace('ChargeResult{', '&ChargeResult{')
with open("rule_charge_result_read.go", "w") as f:
    f.write(text)

with open("rule_payment_status_result_read.go", "r") as f:
    text = f.read()
text = text.replace('PaymentStatusResult{Id: pay.Id, Status: pay.Status}', '&PaymentStatusResult{Id: pay.Id, Status: pay.Status}')
with open("rule_payment_status_result_read.go", "w") as f:
    f.write(text)

with open("rule_refund_create.go", "r") as f:
    text = f.read()
text = text.replace('p.Status = StatusPending', '')
with open("rule_refund_create.go", "w") as f:
    f.write(text)

with open("view.go", "r") as f:
    text = f.read()
text = text.replace('reg.OperationName(ModelName, OpListPayments)', '""')
with open("view.go", "w") as f:
    f.write(text)
