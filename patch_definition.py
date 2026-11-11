import re

with open("definition.go", "r") as f:
    text = f.read()

text = text.replace('{Name: "id", Type: model.Text()},', '{Name: "id", Type: model.Text()},\n\t\t{Name: "tenant_id", Type: model.Text(), NotNull: true},')
with open("definition.go", "w") as f:
    f.write(text)
