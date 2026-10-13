//go:build !wasm

package ui

import (
	"webtyp.com/svg"
	"webtyp.com/svg/sprite"
)

// Icons aloja IconSvg para su descubrimiento por la herramienta de assets.
type Icons struct{}

func (m *Icons) IconSvg() *sprite.Sprite {
	return sprite.NewSprite(
		// Tarjeta de pago: "Pagos".
		sprite.Define(svg.Icon(ID), "0 0 16 16",
			sprite.Path("M1 3.5A1.5 1.5 0 0 1 2.5 2h11A1.5 1.5 0 0 1 15 3.5V5H1V3.5zM1 7h14v5.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 1 12.5V7zm2 3v1h4v-1H3z"),
		),
	)
}
