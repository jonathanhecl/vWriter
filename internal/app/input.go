package app

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/jonathanhecl/vWriter/internal/prompt"
)

// layoutInput renders the left column: media, duration, aspect ratio, brief,
// advanced runtime, and the system prompt.
func (a *App) layoutInput(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: 12, Bottom: 8, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.leftList.Layout(gtx, 12, func(gtx layout.Context, index int) layout.Dimensions {
			switch index {
			case 0:
				return a.layoutMediaSection(gtx)
			case 1:
				return a.layoutTargetSection(gtx)
			case 2:
				return a.layoutBrief(gtx)
			case 3:
				return a.layoutAdvanced(gtx)
			case 4:
				return a.layoutSystemPrompt(gtx)
			case 5:
				return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{}
				})
			}
			return layout.Dimensions{}
		})
	})
}

var aspectDisplayNames = []string{
	"16:9  Widescreen",
	"9:16  Vertical",
	"1:1  Square",
	"4:3  Standard",
	"3:4  Portrait",
	"3:2  Classic",
	"2:3  Book",
	"21:9  Ultrawide",
}

// layoutTargetSection renders the duration slider and aspect ratio picker side-by-side.
func (a *App) layoutTargetSection(gtx layout.Context) layout.Dimensions {
	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			// DURATION (Left half)
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return sectionLabel(gtx, a.theme, "DURATION")
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, 13, fmt.Sprintf("%d seconds", a.durationSeconds()))
									l.Color = colorText
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Background{}.Layout(gtx,
									func(gtx layout.Context) layout.Dimensions {
										bordered(gtx, 6, colorSurface, colorBorder)
										return layout.Dimensions{Size: gtx.Constraints.Min}
									},
									func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Left: 10, Right: 10, Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											slider := material.Slider(a.theme, &a.duration)
											slider.Color = colorAccent
											return slider.Layout(gtx)
										})
									},
								)
							})
						}),
					)
				})
			}),
			// ASPECT RATIO (Right half)
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return sectionLabel(gtx, a.theme, "ASPECT RATIO")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							chosen, changed := a.aspectDropdown.Layout(gtx, a.theme, aspectDisplayNames, a.aspectIndex)
							if changed {
								a.aspectIndex = chosen
								a.cfg.AspectRatio = aspectOptions[chosen]
								a.saveConfig()
							}
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(36))}
						})
					}),
				)
			}),
		)
	})
}

// layoutBrief renders the creative brief editor with a character counter.
func (a *App) layoutBrief(gtx layout.Context) layout.Dimensions {
	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		count := len(a.briefEditor.Text())
		countColor := colorTextDim
		if count > 2000 {
			countColor = colorDanger
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return sectionLabel(gtx, a.theme, "CREATIVE BRIEF")
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 12, "Describe what should happen in the video")
						l.Color = colorTextDim
						return l.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.Y = gtx.Dp(112)
							return a.multilineBox(gtx, &a.briefEditor, "Describe the video and the role of each reference.")
						}),
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Bottom: 8, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, 11, fmt.Sprintf("%d / 2,000", count))
									l.Color = countColor
									return l.Layout(gtx)
								})
							})
						}),
					)
				})
			}),
		)
	})
}

// layoutAdvanced renders the collapsible runtime controls.
func (a *App) layoutAdvanced(gtx layout.Context) layout.Dimensions {
	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.collapsibleHeader(gtx, &a.advOpen, "Advanced runtime")
			}),
		}
		if a.advOpen.Value {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return bodyText(gtx, a.theme, "Context")
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Dp(180)
								gtx.Constraints.Max.X = gtx.Constraints.Min.X
								chosen, changed := a.contextDropdown.Layout(gtx, a.theme, contextOptions, a.contextIndex)
								if changed {
									a.contextIndex = chosen
									a.cfg.ContextProfile = contextProfiles[chosen]
									a.saveConfig()
								}
								return layout.Dimensions{Size: image.Pt(gtx.Dp(180), gtx.Dp(34))}
							}),
						)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						cb := material.CheckBox(a.theme, &a.thinking, "Thinking (slower, more deliberate)")
						cb.Color = colorText
						return cb.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						cb := material.CheckBox(a.theme, &a.keepLoaded, "Keep model loaded between generations")
						cb.Color = colorText
						return cb.Layout(gtx)
					})
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// layoutSystemPrompt renders the collapsible system prompt editor.
func (a *App) layoutSystemPrompt(gtx layout.Context) layout.Dimensions {
	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.collapsibleHeader(gtx, &a.sysOpen, "System prompt")
			}),
		}
		if a.sysOpen.Value {
			custom := strings.TrimSpace(a.sysEditor.Text()) != ""
			status := "Default"
			if custom {
				status = "Custom"
			}
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, 12, status)
								l.Color = colorTextDim
								return l.Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								count := len(a.sysEditor.Text())
								l := material.Label(a.theme, 12, fmt.Sprintf("%d / %s", count, formatK(prompt.MaxSystemPromptChars)))
								if count > prompt.MaxSystemPromptChars {
									l.Color = colorDanger
								} else {
									l.Color = colorTextDim
								}
								return l.Layout(gtx)
							}),
						)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = gtx.Dp(80)
						return a.multilineBox(gtx, &a.sysEditor,
							"Leave empty to use the built-in full-reference instruction.")
					})
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// collapsibleHeader renders a toggleable section header.
func (a *App) collapsibleHeader(gtx layout.Context, state *widget.Bool, title string) layout.Dimensions {
	return state.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				chevron := "›"
				if state.Value {
					chevron = "v"
				}
				l := material.Label(a.theme, 18, chevron)
				l.Color = colorAccent
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 7}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 14, title)
					l.Color = colorText
					return l.Layout(gtx)
				})
			}),
		)
	})
}

// multilineBox renders a multiline editor with a default box height that
// automatically expands vertically as text or enter lines are added.
func (a *App) multilineBox(gtx layout.Context, editor *widget.Editor, hint string) layout.Dimensions {
	padding := gtx.Dp(8)

	minWidth := gtx.Constraints.Min.X
	maxWidth := gtx.Constraints.Max.X
	if maxWidth == 0 {
		maxWidth = minWidth
	}
	if minWidth == 0 {
		minWidth = maxWidth
	}

	minHeight := gtx.Constraints.Min.Y
	if minHeight == 0 {
		minHeight = gtx.Dp(112)
	}
	maxHeight := gtx.Constraints.Max.Y
	if maxHeight < minHeight {
		maxHeight = 1e6
	}

	minInnerWidth := max(minWidth-padding*2, 0)
	maxInnerWidth := max(maxWidth-padding*2, 0)
	minInnerHeight := max(minHeight-padding*2, 0)
	maxInnerHeight := max(maxHeight-padding*2, minInnerHeight)

	// Record editor drawing operations first to measure actual text height.
	macro := op.Record(gtx.Ops)
	editorGtx := gtx
	editorGtx.Constraints.Min = image.Pt(minInnerWidth, minInnerHeight)
	editorGtx.Constraints.Max = image.Pt(maxInnerWidth, maxInnerHeight)
	ed := material.Editor(a.theme, editor, hint)
	ed.Color = colorText
	ed.HintColor = colorTextDim
	ed.TextSize = 13
	dims := ed.Layout(editorGtx)
	call := macro.Stop()

	// Compute final box dimensions.
	finalWidth := max(dims.Size.X+padding*2, minWidth)
	finalHeight := max(dims.Size.Y+padding*2, minHeight)

	// Draw rounded background box.
	bgGtx := gtx
	bgGtx.Constraints.Min = image.Pt(finalWidth, finalHeight)
	bgGtx.Constraints.Max = image.Pt(finalWidth, finalHeight)
	bordered(bgGtx, 6, colorCard, colorBorder)

	// Replay editor drawing operations with padding offset.
	offset := op.Offset(image.Pt(padding, padding)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()

	return layout.Dimensions{Size: image.Pt(finalWidth, finalHeight)}
}

func formatK(value int) string {
	return fmt.Sprintf("%d,%03d", value/1000, value%1000)
}
