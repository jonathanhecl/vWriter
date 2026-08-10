package app

import (
	"image/color"

	"gioui.org/widget/material"
)

// Dark palette for the whole app.
var (
	colorBackground = color.NRGBA{R: 0x0d, G: 0x0f, B: 0x14, A: 0xff}
	colorSurface    = color.NRGBA{R: 0x16, G: 0x19, B: 0x21, A: 0xff}
	colorCard       = color.NRGBA{R: 0x1d, G: 0x21, B: 0x2b, A: 0xff}
	colorBorder     = color.NRGBA{R: 0x2e, G: 0x33, B: 0x40, A: 0xff}
	colorText       = color.NRGBA{R: 0xe6, G: 0xe9, B: 0xef, A: 0xff}
	colorTextDim    = color.NRGBA{R: 0x9a, G: 0xa1, B: 0xaf, A: 0xff}
	colorAccent     = color.NRGBA{R: 0x5c, G: 0x8d, B: 0xff, A: 0xff}
	colorAccentDark = color.NRGBA{R: 0x2b, G: 0x4d, B: 0x99, A: 0xff}
	colorDanger     = color.NRGBA{R: 0xe0, G: 0x5c, B: 0x5c, A: 0xff}
	colorOK         = color.NRGBA{R: 0x5c, G: 0xc0, B: 0x8a, A: 0xff}
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
