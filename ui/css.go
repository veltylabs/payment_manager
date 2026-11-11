//go:build !wasm

package ui

import (
	"webtyp.com/css"
	"webtyp.com/widget"
	"webtyp.com/widget/style"
)

// RenderCSS es el paso de pago: monto destacado, error visible, cuerpo en
// columna, el campo con la superficie de formulario y el botón al inicio (fila).
func (s *PaymentStep) RenderCSS() *css.Stylesheet {
	return style.For(s).
		Root(style.Stack(style.Space3)).
		Part(PartAmount, style.FontSize(style.TextLg), style.FontWeight(style.WeightBold)).
		Part(PartError, style.As(style.DangerWash), style.FontSize(style.TextSm)).
		Part(PartBody, style.Stack(style.Space3)).
		Part(PartInput, style.As(style.Field), style.Round(style.RadiusMd), style.PadInline(style.Space3), style.ControlBox()).
		Part(PartActions, style.Row(style.Space2)).
		Part(PartPay, style.Button(style.Primary)).
		When(widget.Busy, PartPay, style.As(style.Inactive)).
		Stylesheet()
}

// RenderCSS es la pantalla Pagos: barra de acciones, mensaje, lista y detalle.
func (v *PaymentsView) RenderCSS() *css.Stylesheet {
	return style.For(v).
		Root(style.Stack(style.Space4), style.Pad(style.Space4)).
		Part(PartToolbar, style.Row(style.Space2)).
		Part(PartMessage, style.FontSize(style.TextSm)).
		Part(PartDetail, style.Stack(style.Space3), style.As(style.Panel), style.Pad(style.Space4)).
		Part(PartRefund, style.Stack(style.Space2)).
		Stylesheet()
}
