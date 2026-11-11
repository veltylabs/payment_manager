import re

with open("module.go", "r") as f:
    text = f.read()

text = re.sub(r'func \(m \*Module\) CreateCounterPayment\(.*?\n.*?(?=func \(m \*Module\))', '', text, flags=re.DOTALL)
text = re.sub(r'func \(m \*Module\) StartOnlinePayment\(.*?\n.*?(?=func \(m \*Module\))', '', text, flags=re.DOTALL)
text = re.sub(r'func \(m \*Module\) StartTerminalPayment\(.*?\n.*?(?=func \(m \*Module\))', '', text, flags=re.DOTALL)
text = re.sub(r'func \(m \*Module\) RefreshTerminalPayment\(.*?\n.*?(?=func \(m \*Module\))', '', text, flags=re.DOTALL)
text = re.sub(r'func \(m \*Module\) RefundPayment\(.*?\n.*?(?=func \(m \*Module\))', '', text, flags=re.DOTALL)

with open("module.go", "w") as f:
    f.write(text)
