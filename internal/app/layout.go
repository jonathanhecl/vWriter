package app

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
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
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Max.X = gtx.Dp(480)
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
	)
}

// layoutHeader renders the title, Ollama URL, and model picker.
func (a *App) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: 10, Bottom: 10, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H6(a.theme, "vWriter")
				title.Color = colorText
				return title.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 16, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return bodyText(gtx, a.theme, "Ollama")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(230)
				gtx.Constraints.Max.X = gtx.Dp(230)
				return a.editorBox(gtx, &a.urlEditor, 30)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.smallButton(gtx, &a.connectBtn, "Connect")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.smallButton(gtx, &a.refreshBtn, "Refresh")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(300)
				return a.layoutModelPicker(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
		)
	})
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
					return bodyText(gtx, a.theme, "Loading models?")
				})
			}),
		)
	}
	if len(models) == 0 {
		text := "No models ? check the URL and press Connect"
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
		options[index] = fmt.Sprintf("%s (%s)", model.Name, model.Size)
		if model.Name == a.cfg.Model {
			selected = index
		}
	}
	chosen, changed := a.modelDropdown.Layout(gtx, a.theme, options, selected)
	if changed {
		a.cfg.Model = models[chosen].Name
		a.saveConfig()
	}
	return layout.Dimensions{Size: image.Pt(gtx.Dp(300), gtx.Dp(34))}
}

// layoutFooter renders the generation status, progress, and memory action.
func (a *App) layoutFooter(gtx layout.Context) layout.Dimensions {
	a.mu.Lock()
	generating := a.generating
	phase := a.phase
	tokens := a.streamTokens
	a.mu.Unlock()

	return layout.Inset{Top: 8, Bottom: 8, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !generating {
					return layout.Dimensions{}
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Loader(a.theme).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							text := phaseLabel(phase)
							if phase == engine.PhaseGenerating && tokens > 0 {
								text = fmt.Sprintf("%s ? %d tokens", text, tokens)
							}
							l := material.Label(a.theme, 13, text)
							l.Color = colorTextDim
							return l.Layout(gtx)
						})
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.smallButton(gtx, &a.unloadBtn, "Unload model")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if generating {
					return a.dangerButton(gtx, &a.cancelBtn, "Cancel")
				}
				return a.primaryButton(gtx, &a.generateBtn, "Generate prompt")
			}),
		)
	})
}

// phaseLabel maps engine phases to display text.
func phaseLabel(phase string) string {
	switch phase {
	case engine.PhaseLoadingModel:
		return "Loading model?"
	case engine.PhaseProcessingMedia:
		return "Processing media?"
	case engine.PhaseGenerating:
		return "Generating?"
	case engine.PhaseRepairing:
		return "Repairing format?"
	case engine.PhaseCancelling:
		return "Cancelling?"
	}
	return phase
}

func (a *App) vDivider(gtx layout.Context) layout.Dimensions {
	width := gtx.Dp(1)
	paint.FillShape(gtx.Ops, colorBorder, clip.Rect(image.Rect(0, 0, width, gtx.Constraints.Max.Y)).Op())
	return layout.Dimensions{Size: image.Pt(width, gtx.Constraints.Max.Y)}
}

// Buttons.

func (a *App) primaryButton(gtx layout.Context, btn *widget.Clickable, label string) layout.Dimensions {
	b := material.Button(a.theme, btn, label)
	b.Background = colorAccent
	return b.Layout(gtx)
}

func (a *App) dangerButton(gtx layout.Context, btn *widget.Clickable, label string) layout.Dimensions {
	b := material.Button(a.theme, btn, label)
	b.Background = colorDanger
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
