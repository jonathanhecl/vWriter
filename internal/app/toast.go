package app

import (
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// toastLifetime is how long a non-error toast stays visible.
const toastLifetime = 5 * time.Second

// toastClick holds the clickable zones of one toast: the dismiss area and the
// copy button shown on error toasts.
type toastClick struct {
	dismiss widget.Clickable
	copy    widget.Clickable
}

// layoutToasts renders queued notifications bottom-right. Non-error toasts
// auto-dismiss after toastLifetime and are dismissed by a click; error toasts
// persist and expose a Copy button that copies the error message.
func (a *App) layoutToasts(gtx layout.Context) layout.Dimensions {
	for len(a.toastClicks) < len(a.toasts) {
		a.toastClicks = append(a.toastClicks, toastClick{})
	}
	a.pruneToasts(time.Now())
	if len(a.toasts) == 0 {
		return layout.Dimensions{}
	}
	for index := range a.toastClicks {
		if index >= len(a.toasts) {
			break
		}
		if a.toasts[index].isError && a.toastClicks[index].copy.Clicked(gtx) {
			a.copyErrorToast(gtx, index)
			return layout.Dimensions{}
		}
		if !a.toasts[index].isError && a.toastClicks[index].dismiss.Clicked(gtx) {
			a.removeToast(index)
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

// pruneToasts removes expired non-error toasts. Error toasts persist.
func (a *App) pruneToasts(now time.Time) {
	for index := 0; index < len(a.toasts); {
		if !a.toasts[index].isError && !a.toasts[index].created.IsZero() && now.Sub(a.toasts[index].created) >= toastLifetime {
			a.removeToast(index)
			continue
		}
		index++
	}
}

// removeToast drops one toast and its clickable state. toastClicks may lag
// behind toasts (it is grown during layout), so removal is guarded.
func (a *App) removeToast(index int) {
	if index < 0 || index >= len(a.toasts) {
		return
	}
	a.toasts = append(a.toasts[:index], a.toasts[index+1:]...)
	if index < len(a.toastClicks) {
		a.toastClicks = append(a.toastClicks[:index], a.toastClicks[index+1:]...)
	}
}

// copyErrorToast copies an error message (text + details) to the clipboard
// and dismisses it.
func (a *App) copyErrorToast(gtx layout.Context, index int) {
	if index < 0 || index >= len(a.toasts) {
		return
	}
	toast := a.toasts[index]
	text := toast.text
	if toast.details != "" {
		text += "\n\n" + toast.details
	}
	gtx.Execute(clipboard.WriteCmd{Data: nopCloser{strings.NewReader(text)}})
	a.pushToast("Error message copied to the clipboard.", "", false)
	a.removeToast(index)
}

func (a *App) layoutToast(gtx layout.Context, index int) layout.Dimensions {
	toast := a.toasts[index]
	gtx.Constraints.Max.X = gtx.Dp(420)
	gtx.Constraints.Min.X = gtx.Dp(280)

	content := func(gtx layout.Context) layout.Dimensions {
		border := colorBorder
		textColor := colorText
		if toast.isError {
			border = colorDanger
			textColor = colorDanger
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				bordered(gtx, 6, colorSurface, border)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(a.theme, toast.text)
							label.Color = textColor
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if toast.details == "" {
								return layout.Dimensions{}
							}
							return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								d := material.Body2(a.theme, toast.details)
								d.Color = colorTextDim
								d.MaxLines = 4
								return d.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !toast.isError {
								return layout.Dimensions{}
							}
							return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.toastClicks[index].copy.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.Background{}.Layout(gtx,
											func(gtx layout.Context) layout.Dimensions {
												bordered(gtx, 4, colorSurface, colorDanger)
												return layout.Dimensions{Size: gtx.Constraints.Min}
											},
											func(gtx layout.Context) layout.Dimensions {
												return layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													l := material.Label(a.theme, 12, "Copy error")
													l.Color = colorDanger
													return l.Layout(gtx)
												})
											},
										)
									})
								})
							})
						}),
					)
				})
			}),
		)
	}

	if toast.isError {
		return content(gtx)
	}
	return a.toastClicks[index].dismiss.Layout(gtx, content)
}
