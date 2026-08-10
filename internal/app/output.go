package app

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget/material"
)

// layoutOutput renders the right column: the generated prompt editor, its
// actions, and the refine bar.
func (a *App) layoutOutput(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: 12, Bottom: 8, Left: 20, Right: 20}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutOutputHeader(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.multilineBox(gtx, &a.outputEditor,
						"The generated prompt appears here. You can edit it before copying.")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutRefineBar(gtx)
			}),
		)
	})
}

// layoutOutputHeader renders the output title, Modified badge, counts, and
// action buttons.
func (a *App) layoutOutputHeader(gtx layout.Context) layout.Dimensions {
	modified := a.hasResult && a.outputEditor.Text() != a.lastAIMark
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return sectionLabel(gtx, a.theme, "Generated full-reference prompt")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !modified {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.theme, 11, "MODIFIED")
				l.Color = colorAccent
				return l.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				text := a.outputEditor.Text()
				words := len(strings.Fields(text))
				return bodyText(gtx, a.theme, fmt.Sprintf("%d chars · %d words", len(text), words))
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if !modified {
					return layout.Dimensions{}
				}
				return a.smallButton(gtx, &a.restoreBtn, "Undo edits")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if !a.hasResult {
					return layout.Dimensions{}
				}
				return a.smallButton(gtx, &a.refineBtn, "Refine")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !a.hasResult {
				return layout.Dimensions{}
			}
			return a.smallButton(gtx, &a.copyBtn, "Copy prompt")
		}),
	)
}

// layoutRefineBar renders the refine instruction editor when open.
func (a *App) layoutRefineBar(gtx layout.Context) layout.Dimensions {
	if !a.refineOpen || !a.hasResult {
		return layout.Dimensions{}
	}
	return card(gtx, 10, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return sectionLabel(gtx, a.theme, "Refine (text only, media is not re-sent)")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.Y = gtx.Dp(40)
							return a.multilineBox(gtx, &a.refineEditor,
								"Revision instruction, e.g. “Make the lighting colder.”")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.primaryButton(gtx, &a.rewriteBtn, "Rewrite")
							})
						}),
					)
				})
			}),
		)
	})
}
