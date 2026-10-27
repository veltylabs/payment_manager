//go:build browser

package main

import (
	"github.com/veltylabs/payment_manager/ui"
	. "webtyp.com/dom"
	"webtyp.com/html"
	"webtyp.com/router/loopback"
)

func main() {
	caller := loopback.New()
	screens := ui.Screens(caller)
	appNode := html.Div().Child(
		html.H1().Text(string(screens[0].Label)),
		html.P().Text("Demo"),
	)
	Append("body", appNode)
	select {}
}