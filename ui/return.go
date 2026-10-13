package ui

import (
	. "webtyp.com/dom"
	. "webtyp.com/html"
)

type TakeReturn struct {
	Element
	OnOnlineReturn func(token string)
}

func NewTakeReturn(onReturn func(token string)) *TakeReturn {
	v := &TakeReturn{
		OnOnlineReturn: onReturn,
	}
	v.Element = *Div()
	v.process()
	return v
}

func (v *TakeReturn) Render() *Element {
	return &v.Element
}

func (v *TakeReturn) process() {
	token, _ := GetQuery("token_ws")
	if token == "" {
		token, _ = GetQuery("TBK_TOKEN")
	}

	if token == "" {
		return
	}

	ClearQuery()

	if v.OnOnlineReturn != nil {
		v.OnOnlineReturn(token)
	}
}
