package app

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/jonathanhecl/vWriter/internal/engine"
)

// layout renders the whole window: header, split panels, footer, overlays.
func (a *App) layout(gtx layout.Context) {
	paint.FillShape(gtx.Ops, colorBackground, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
	layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.layoutHeader(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					leftWidth := a.cfg.LeftPanelWidth
					if leftWidth < 300 {
						leftWidth = 300
					} else if leftWidth > 850 {
						leftWidth = 850
					}
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Max.X = gtx.Dp(unit.Dp(leftWidth))
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return a.layoutInput(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.vDivider(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.layoutOutput(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.layoutFooter(gtx) }),
			)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions { return a.layoutToasts(gtx) }),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions { return a.layoutModal(gtx) }),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions { return a.layoutSavePresetModal(gtx) }),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions { return a.layoutAssetRoleModal(gtx) }),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions { return a.layoutBusy(gtx) }),
	)
}

// layoutBusy renders a full-window blocking overlay while any generation
// (generate, extend, regenerate, or refine) is running. It claims all pointer
// events so no other control is usable until the operation finishes or fails.
func (a *App) layoutBusy(gtx layout.Context) layout.Dimensions {
	a.mu.Lock()
	generating := a.generating
	phase := a.phase
	tokens := a.streamTokens
	a.mu.Unlock()
	if !generating {
		return layout.Dimensions{}
	}

	full := image.Rectangle{Max: gtx.Constraints.Max}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 0x99}, clip.Rect(full).Op())
	defer clip.Rect(full).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &a.busyTag)
	for {
		_, ok := gtx.Event(pointer.Filter{Target: &a.busyTag, Kinds: pointer.Press | pointer.Drag | pointer.Scroll})
		if !ok {
			break
		}
	}

	// Center the card within the whole window: layout.Center only centers
	// inside the minimum constraints, so pin Min to the window size first.
	cgtx := gtx
	cgtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	return layout.Center.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(340)
		gtx.Constraints.Min.X = gtx.Dp(280)
		return card(gtx, 16, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					loaderGtx := gtx
					loaderGtx.Constraints.Min = image.Pt(gtx.Dp(28), gtx.Dp(28))
					loaderGtx.Constraints.Max = loaderGtx.Constraints.Min
					l := material.Loader(a.theme)
					l.Color = colorAccent
					return l.Layout(loaderGtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						text := phaseLabel(phase)
						if phase == engine.PhaseGenerating && tokens > 0 {
							text = fmt.Sprintf("%s · %d tokens", text, tokens)
						}
						l := material.Label(a.theme, 14, text)
						l.Color = colorText
						return l.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.dangerButton(gtx, &a.cancelBtn, "Cancel")
					})
				}),
			)
		})
	})
}

func (a *App) layoutSavePresetModal(gtx layout.Context) layout.Dimensions {
	if !a.savingPreset {
		return layout.Dimensions{}
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fill(gtx, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 0xb0})
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			cgtx := gtx
			cgtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
			return layout.Center.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Pt(gtx.Dp(360), 0)
				gtx.Constraints.Max.X = gtx.Dp(360)
				return card(gtx, 18, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, 16, "Save Creative Template")
							l.Color = colorText
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 4, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, 12, "Enter a name for this prompt template:")
								l.Color = colorTextDim
								return l.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.singlelineBox(gtx, &a.presetNameEditor, "Template name (e.g. Action Motion Reference)")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.smallButton(gtx, &a.cancelSaveBtn, "Cancel")
										})
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.smallButton(gtx, &a.confirmSaveBtn, "Save")
									}),
								)
							})
						}),
					)
				})
			})
		}),
	)
}

// layoutHeader renders the top bar: brand mark, Ollama endpoint controls, and model controls.
// layoutHeader renders the top bar: brand mark, Ollama endpoint controls, and model controls.
func (a *App) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: 12, Bottom: 12, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutBrand(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 24, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return sectionLabel(gtx, a.theme, "Ollama endpoint")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(180)
				gtx.Constraints.Max.X = gtx.Dp(180)
				return a.editorBox(gtx, &a.urlEditor, 34)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 6, Right: 24}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.smallButton(gtx, &a.connectBtn, "Fetch models")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return sectionLabel(gtx, a.theme, "Model")
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.layoutModelPicker(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.smallButton(gtx, &a.unloadBtn, "Unload")
				})
			}),
		)
	})
}

// layoutBrand renders the compact product mark and its descriptor.
func (a *App) layoutBrand(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Dp(30)
			gtx.Constraints.Min = image.Pt(size, size)
			gtx.Constraints.Max = image.Pt(size, size)
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					fill(gtx, 9, colorAccent)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 17, "V")
						l.Color = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
						return l.Layout(gtx)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 9}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, 18, "vWriter")
						l.Color = colorText
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return bodyText(gtx, a.theme, "VIDEO PROMPT STUDIO") }),
				)
			})
		}),
	)
}

// formatModelDisplay formats a model entry so that the end of the name
// (model variant, tag, quantization, size) is always preserved and visible.
func formatModelDisplay(name, size string, maxLen int) string {
	cleaned := strings.TrimPrefix(name, "hf.co/")
	suffix := ""
	if size != "" {
		suffix = fmt.Sprintf(" (%s)", size)
	}

	total := cleaned + suffix
	if len([]rune(total)) <= maxLen {
		return total
	}

	// Try stripping author/username prefix if present.
	if slashIdx := strings.Index(cleaned, "/"); slashIdx >= 0 {
		withoutUser := cleaned[slashIdx+1:]
		totalNoUser := withoutUser + suffix
		if len([]rune(totalNoUser)) <= maxLen {
			return totalNoUser
		}
		cleaned = withoutUser
	}

	targetLen := maxLen - len([]rune(suffix)) - 1
	runesCleaned := []rune(cleaned)
	if len(runesCleaned) > targetLen && targetLen > 0 {
		cleaned = "…" + string(runesCleaned[len(runesCleaned)-targetLen:])
	}

	return cleaned + suffix
}

// layoutModelPicker renders the model dropdown and its status.
func (a *App) layoutModelPicker(gtx layout.Context) layout.Dimensions {
	a.mu.Lock()
	loading := a.modelsLoading
	models := append([]modelEntry(nil), a.models...)
	modelsErr := a.modelsError
	a.mu.Unlock()

	if loading {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Loader(a.theme).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return bodyText(gtx, a.theme, "Loading models…")
				})
			}),
		)
	}
	if len(models) == 0 {
		text := "No models — click Fetch models"
		if modelsErr != "" {
			text = modelsErr
		}
		l := material.Label(a.theme, 13, text)
		l.Color = colorDanger
		return l.Layout(gtx)
	}
	options := make([]string, len(models))
	selected := 0
	for index, model := range models {
		options[index] = formatModelDisplay(model.Name, model.Size, 70)
		if model.Name == a.cfg.Model {
			selected = index
		}
	}
	chosen, changed := a.modelDropdown.Layout(gtx, a.theme, options, selected)
	if changed {
		a.cfg.Model = models[chosen].Name
		a.saveConfig()
	}
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Dp(34))}
}

// layoutFooter renders the generation status, progress, and memory action.
func (a *App) layoutFooter(gtx layout.Context) layout.Dimensions {
	a.mu.Lock()
	generating := a.generating
	phase := a.phase
	tokens := a.streamTokens
	a.mu.Unlock()

	return layout.Inset{Top: 10, Bottom: 14, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !generating {
					return layout.Dimensions{}
				}
				return layout.Background{}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						bordered(gtx, 6, colorSurface, colorBorder)
						return layout.Dimensions{Size: gtx.Constraints.Min}
					},
					func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									loaderGtx := gtx
									loaderGtx.Constraints.Max = image.Pt(gtx.Dp(16), gtx.Dp(16))
									loaderGtx.Constraints.Min = loaderGtx.Constraints.Max
									return material.Loader(a.theme).Layout(loaderGtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										text := phaseLabel(phase)
										if phase == engine.PhaseGenerating && tokens > 0 {
											text = fmt.Sprintf("%s · %d tokens", text, tokens)
										}
										l := material.Label(a.theme, 12, text)
										l.Color = colorText
										return l.Layout(gtx)
									})
								}),
							)
						})
					},
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if generating {
						return layout.Dimensions{}
					}
					return a.primaryButton(gtx, &a.generateBtn, "Generate prompt")
				})
			}),
		)
	})
}

// phaseLabel maps engine phases to display text.
func phaseLabel(phase string) string {
	switch phase {
	case engine.PhaseLoadingModel:
		return "Loading model…"
	case engine.PhaseProcessingMedia:
		return "Processing media…"
	case engine.PhaseGenerating:
		return "Generating…"
	case engine.PhaseRepairing:
		return "Repairing format…"
	case engine.PhaseCancelling:
		return "Cancelling…"
	}
	return phase
}

func (a *App) vDivider(gtx layout.Context) layout.Dimensions {
	hitWidth := gtx.Dp(8)
	lineWidth := gtx.Dp(1)
	lineOffset := (hitWidth - lineWidth) / 2

	rect := clip.Rect(image.Rect(0, 0, hitWidth, gtx.Constraints.Max.Y)).Push(gtx.Ops)
	a.dividerDrag.Add(gtx.Ops)
	pointer.CursorColResize.Add(gtx.Ops)
	rect.Pop()

	// Process drag events on divider. gesture.Drag reports absolute pointer
	// positions within the hit area (not per-event deltas), so the width is
	// computed from the press position and the width captured at press time;
	// the right-edge clamp uses the window width, not the divider's own
	// remaining constraint.
	for {
		event, ok := a.dividerDrag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		switch event.Kind {
		case pointer.Press:
			a.dividerPressX = event.Position.X
			a.dividerStartWidth = a.cfg.LeftPanelWidth
		case pointer.Drag:
			deltaDp := int((event.Position.X - a.dividerPressX) / float32(gtx.Dp(1)))
			if deltaDp != 0 {
				maxLeft := max(a.lastWidthDp-250, 250)
				newWidth := min(max(a.dividerStartWidth+deltaDp, 250), maxLeft)
				if newWidth != a.cfg.LeftPanelWidth {
					a.cfg.LeftPanelWidth = newWidth
					a.window.Invalidate()
				}
			}
		case pointer.Release, pointer.Cancel:
			a.saveConfig()
		}
	}

	// Draw 1dp line centered in 8dp hit area
	off := op.Offset(image.Pt(lineOffset, 0)).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, colorBorder, clip.Rect(image.Rect(0, 0, lineWidth, gtx.Constraints.Max.Y)).Op())
	off.Pop()

	return layout.Dimensions{Size: image.Pt(hitWidth, gtx.Constraints.Max.Y)}
}

// Buttons.

func (a *App) primaryButton(gtx layout.Context, btn *widget.Clickable, label string) layout.Dimensions {
	b := material.Button(a.theme, btn, label)
	b.Background = colorAccent
	b.TextSize = 13
	b.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 16, Right: 16}
	b.CornerRadius = unit.Dp(6)
	return b.Layout(gtx)
}

func (a *App) dangerButton(gtx layout.Context, btn *widget.Clickable, label string) layout.Dimensions {
	b := material.Button(a.theme, btn, label)
	b.Background = colorDanger
	b.TextSize = 13
	b.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 16, Right: 16}
	b.CornerRadius = unit.Dp(6)
	return b.Layout(gtx)
}

func (a *App) smallButton(gtx layout.Context, btn *widget.Clickable, label string) layout.Dimensions {
	b := material.Button(a.theme, btn, label)
	b.Background = colorCard
	b.Color = colorText
	b.TextSize = 12
	return b.Layout(gtx)
}

// editorBox renders a single-line editor in a bordered box of fixed height.
func (a *App) editorBox(gtx layout.Context, editor *widget.Editor, height unit.Dp) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(height)
	gtx.Constraints.Max.Y = gtx.Dp(height)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bordered(gtx, 6, colorCard, colorBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(a.theme, editor, "")
				ed.Color = colorText
				ed.TextSize = 13
				return ed.Layout(gtx)
			})
		}),
	)
}

var _ = strings.ToUpper
