package app

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// toastClick is the dismissal click zone of one toast.
type toastClick struct {
	widget.Clickable
}

// layoutToasts renders queued notifications bottom-right. Each toast is
// dismissed by a click.
func (a *App) layoutToasts(gtx layout.Context) layout.Dimensions {
	if len(a.toasts) == 0 {
		return layout.Dimensions{}
	}
	for len(a.toastClicks) < len(a.toasts) {
		a.toastClicks = append(a.toastClicks, toastClick{})
	}
	for index := range a.toastClicks {
		if index < len(a.toasts) && a.toastClicks[index].Clicked(gtx) {
			a.toasts = append(a.toasts[:index], a.toasts[index+1:]...)
			a.toastClicks = append(a.toastClicks[:index], a.toastClicks[index+1:]...)
			return layout.Dimensions{}
		}
	}
	return layout.Inset{Top: 0, Bottom: 48, Left: 0, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(a.toasts))
			for index := range a.toasts {
				index := index
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutToast(gtx, index)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (a *App) layoutToast(gtx layout.Context, index int) layout.Dimensions {
	toast := a.toasts[index]
	gtx.Constraints.Max.X = gtx.Dp(420)
	gtx.Constraints.Min.X = gtx.Dp(280)
	return a.toastClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		border := colorBorder
		if toast.isError {
			border = colorDanger
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				bordered(gtx, 6, colorSurface, border)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(a.theme, toast.text)
					label.Color = colorText
					return label.Layout(gtx)
				})
			}),
		)
	})
}
