package app

import (
	"image"
	"image/color"
	"io"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// nopCloser adapts a string to io.ReadCloser for clipboard writes.
type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }

var _ io.ReadCloser = nopCloser{}

// fill paints a rounded rectangle background.
func fill(gtx layout.Context, radius unit.Dp, bg color.NRGBA) {
	bounds := image.Rectangle{Max: gtx.Constraints.Min}
	rect := clip.UniformRRect(bounds, gtx.Dp(radius))
	paint.FillShape(gtx.Ops, bg, rect.Op(gtx.Ops))
}

// bordered paints a rounded background with a 1px border.
func bordered(gtx layout.Context, radius unit.Dp, bg, border color.NRGBA) {
	fill(gtx, radius, border)
	off := op.Offset(image.Pt(1, 1)).Push(gtx.Ops)
	size := gtx.Constraints.Min
	size.X -= 2
	size.Y -= 2
	rect := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radius))
	paint.FillShape(gtx.Ops, bg, rect.Op(gtx.Ops))
	off.Pop()
}

// sectionLabel renders a small dim section heading.
func sectionLabel(gtx layout.Context, th *material.Theme, text string) layout.Dimensions {
	label := material.Label(th, 12, strings.ToUpper(text))
	label.Color = colorTextDim
	return label.Layout(gtx)
}

// bodyText renders dimmed helper text.
func bodyText(gtx layout.Context, th *material.Theme, text string) layout.Dimensions {
	label := material.Label(th, 13, text)
	label.Color = colorTextDim
	return label.Layout(gtx)
}

// dropdown is a minimal selection menu. The open menu is deferred so it
// paints above the content that follows the button.
type dropdown struct {
	open  bool
	btn   widget.Clickable
	items []widget.Clickable
}

// Layout renders the dropdown and reports a new selection.
func (d *dropdown) Layout(gtx layout.Context, th *material.Theme, options []string, selected int) (int, bool) {
	for len(d.items) < len(options) {
		d.items = append(d.items, widget.Clickable{})
	}
	chosen, changed := selected, false
	if d.btn.Clicked(gtx) {
		d.open = !d.open
	}
	for index := range options {
		if d.open && d.items[index].Clicked(gtx) {
			chosen, changed, d.open = index, true, false
		}
	}

	label := "Select…"
	if selected >= 0 && selected < len(options) {
		label = options[selected]
	}
	dims := layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bordered(gtx, 6, colorCard, colorBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, 14, label)
						l.Color = colorText
						return l.Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, 14, "▾")
						l.Color = colorTextDim
						return l.Layout(gtx)
					}),
				)
			})
		}),
	)
	dims = d.btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return dims })

	if d.open {
		buttonWidth := dims.Size.X
		macro := op.Record(gtx.Ops)
		off := op.Offset(image.Pt(0, dims.Size.Y+4)).Push(gtx.Ops)
		menuGtx := gtx
		menuGtx.Constraints.Min = image.Pt(buttonWidth, 0)
		menuGtx.Constraints.Max.X = buttonWidth
		d.layoutMenu(menuGtx, th, options, selected)
		off.Pop()
		op.Defer(gtx.Ops, macro.Stop())
	}
	return chosen, changed
}

func (d *dropdown) layoutMenu(gtx layout.Context, th *material.Theme, options []string, selected int) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bordered(gtx, 6, colorSurface, colorBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, len(options))
				for index, option := range options {
					index, option := index, option
					children[index] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return d.items[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Label(th, 14, option)
								if index == selected {
									l.Color = colorAccent
								} else {
									l.Color = colorText
								}
								return l.Layout(gtx)
							})
						})
					})
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}),
	)
}

// card wraps content in a rounded surface with padding.
func card(gtx layout.Context, padding unit.Dp, content layout.Widget) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			bordered(gtx, 10, colorSurface, colorBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(padding).Layout(gtx, content)
		},
	)
}
