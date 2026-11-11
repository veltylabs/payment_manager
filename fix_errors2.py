with open("errors.go", "r") as f:
    text = f.read()

text = text.replace('const (\\n\\tErrTimeoutOutOfRange', 'var (\\n\\tErrTimeoutOutOfRange')
text = text.replace('const (\\n\\tErrTimeoutOutOfRange   = fmt.Err', 'var (\\n\\tErrTimeoutOutOfRange   = fmt.Err')
with open("errors.go", "w") as f:
    f.write(text)
