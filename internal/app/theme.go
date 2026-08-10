package app

import (
	"image/color"

	"gioui.org/widget/material"
)

// Dark palette for the whole app.
var (
	colorBackground = color.NRGBA{R: 0x0b, G: 0x0d, B: 0x12, A: 0xff}
	colorSurface    = color.NRGBA{R: 0x14, G: 0x18, B: 0x22, A: 0xff}
	colorCard       = color.NRGBA{R: 0x1b, G: 0x21, B: 0x2e, A: 0xff}
	colorBorder     = color.NRGBA{R: 0x30, G: 0x39, B: 0x4b, A: 0xff}
	colorText       = color.NRGBA{R: 0xed, G: 0xf0, B: 0xf6, A: 0xff}
	colorTextDim    = color.NRGBA{R: 0x9c, G: 0xa7, B: 0xb9, A: 0xff}
	colorAccent     = color.NRGBA{R: 0x49, G: 0x8a, B: 0xff, A: 0xff}
	colorAccentDark = color.NRGBA{R: 0x20, G: 0x4d, B: 0x9a, A: 0xff}
	colorDanger     = color.NRGBA{R: 0xf0, G: 0x6a, B: 0x6a, A: 0xff}
	colorOK         = color.NRGBA{R: 0x56, G: 0xd3, B: 0x9b, A: 0xff}
)

// newTheme builds the material theme with the dark palette.
func newTheme() *material.Theme {
	theme := material.NewTheme()
	theme.Palette = material.Palette{
		Bg:         colorBackground,
		Fg:         colorText,
		ContrastBg: colorAccent,
		ContrastFg: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	}
	theme.TextSize = 14
	return theme
}
