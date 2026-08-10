package app

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
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

// layoutTargetSection renders the duration slider and aspect ratio picker.
func (a *App) layoutTargetSection(gtx layout.Context) layout.Dimensions {
	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return sectionLabel(gtx, a.theme, "Duration")
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 13, fmt.Sprintf("%d s", a.durationSeconds()))
						l.Color = colorText
						return l.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					slider := material.Slider(a.theme, &a.duration)
					slider.Color = colorAccent
					return slider.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return sectionLabel(gtx, a.theme, "Aspect ratio")
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(120)
						gtx.Constraints.Max.X = gtx.Constraints.Min.X
						chosen, changed := a.aspectDropdown.Layout(gtx, a.theme, aspectOptions, a.aspectIndex)
						if changed {
							a.aspectIndex = chosen
							a.cfg.AspectRatio = aspectOptions[chosen]
							a.saveConfig()
						}
						return layout.Dimensions{Size: image.Pt(gtx.Dp(120), gtx.Dp(34))}
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
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return sectionLabel(gtx, a.theme, "Creative direction")
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 12, fmt.Sprintf("%d / 2,000", count))
						l.Color = countColor
						return l.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Dp(96)
					return a.multilineBox(gtx, &a.briefEditor,
						"Describe the video and the role of each reference.")
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
					chevron = "⌄"
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

// multilineBox renders a multiline editor inside a bordered box.
func (a *App) multilineBox(gtx layout.Context, editor *widget.Editor, hint string) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bordered(gtx, 6, colorCard, colorBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(a.theme, editor, hint)
				ed.Color = colorText
				ed.HintColor = colorTextDim
				ed.TextSize = 13
				return ed.Layout(gtx)
			})
		}),
	)
}

func formatK(value int) string {
	return fmt.Sprintf("%d,%03d", value/1000, value%1000)
}
