package app

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/text"
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
				a.mu.Lock()
				generating := a.generating
				phase := a.phase
				tokens := a.streamTokens
				a.mu.Unlock()

				if generating && strings.TrimSpace(a.outputEditor.Text()) == "" {
					return a.layoutOutputGenerating(gtx, phase, tokens)
				}
				if !a.hasResult && strings.TrimSpace(a.outputEditor.Text()) == "" {
					return a.layoutOutputEmpty(gtx)
				}
				if a.highlightMode && a.hasResult {
					return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return card(gtx, 10, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								list := material.List(a.theme, &a.outputList)
								list.Indicator.Color = colorAccent
								return list.Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
									return a.layoutHighlightedOutput(gtx)
								})
							})
						})
					})
				}
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

// layoutOutputGenerating renders a centered loading card inside the prompt workspace while generating.
func (a *App) layoutOutputGenerating(gtx layout.Context, phase string, tokens int) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(440)
		return card(gtx, 22, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						loaderGtx := gtx
						loaderGtx.Constraints.Min = image.Pt(gtx.Dp(24), gtx.Dp(24))
						loaderGtx.Constraints.Max = loaderGtx.Constraints.Min
						l := material.Loader(a.theme)
						l.Color = colorAccent
						return l.Layout(loaderGtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 15, "Generating video prompt...")
						l.Color = colorText
						return l.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if phase == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 13, phase)
						l.Color = colorTextDim
						return l.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if tokens <= 0 {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 12, fmt.Sprintf("%d tokens generated", tokens))
						l.Color = colorAccent
						return l.Layout(gtx)
					})
				}),
			)
		})
	})
}

// layoutOutputEmpty renders an empty workspace placeholder when no prompt is available.
func (a *App) layoutOutputEmpty(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(420)
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.theme, 26, "✨")
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 16, "No prompt generated yet")
					l.Color = colorText
					return l.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 13, "Add your media references and creative brief, then click Generate Prompt below.")
					l.Color = colorTextDim
					l.Alignment = text.Middle
					return l.Layout(gtx)
				})
			}),
		)
	})
}

// layoutOutputHeader renders the output title, Modified badge, counts, and
// action buttons — always on two separate rows so buttons never get squished.
func (a *App) layoutOutputHeader(gtx layout.Context) layout.Dimensions {
	modified := a.hasResult && a.outputEditor.Text() != a.lastAIMark
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,

		// Row 1: title + MODIFIED badge + char/word count
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
			)
		}),

		// Row 2: action buttons (only when there's a result)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !a.hasResult {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !modified {
							return layout.Dimensions{}
						}
						return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.smallButton(gtx, &a.restoreBtn, "Undo edits")
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.smallButton(gtx, &a.refineBtn, "Refine")
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.smallButton(gtx, &a.copyBtn, "Copy prompt")
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if a.highlightBtn.Clicked(gtx) {
							a.highlightMode = !a.highlightMode
						}
						label := "Show"
						if a.highlightMode {
							label = "Edit"
						}
						return a.smallButton(gtx, &a.highlightBtn, label)
					}),
				)
			})
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
