with open("errors.go", "r") as f:
    text = f.read()

text = text.replace('const (\\n\\tErrTimeoutOutOfRange', 'var (\\n\\tErrTimeoutOutOfRange')
with open("errors.go", "w") as f:
    f.write(text)
