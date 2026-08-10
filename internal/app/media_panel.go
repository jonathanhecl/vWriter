package app

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/jonathanhecl/vWriter/internal/media"
)

// assetWidgets holds the per-asset widget state.
type assetWidgets struct {
	preview  widget.Clickable
	remove   widget.Clickable
	up       widget.Clickable
	down     widget.Clickable
	analysis widget.Clickable
}

func (a *App) widgetsFor(assetID string) *assetWidgets {
	if a.assetWidgetSet == nil {
		a.assetWidgetSet = map[string]*assetWidgets{}
	}
	widgets, ok := a.assetWidgetSet[assetID]
	if !ok {
		widgets = &assetWidgets{}
		a.assetWidgetSet[assetID] = widgets
	}
	return widgets
}

// layoutMediaSection renders the media header and the asset cards.
func (a *App) layoutMediaSection(gtx layout.Context) layout.Dimensions {
	assets := a.engine.Store.List(a.session)
	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return sectionLabel(gtx, a.theme, fmt.Sprintf("Reference media (%d/12)", len(assets)))
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(assets) == 0 {
							return layout.Dimensions{}
						}
						return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.smallButton(gtx, &a.clearMediaBtn, "Clear")
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.smallButton(gtx, &a.addMediaBtn, "Add media")
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return bodyText(gtx, a.theme, "Up to 9 images, 3 videos, and 3 audio files. Clips must be 2–15 seconds.")
				})
			}),
		}
		for index, asset := range assets {
			index, asset := index, asset
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.layoutAssetCard(gtx, asset, index, len(assets))
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// layoutAssetCard renders one asset: thumbnail, reference, actions.
func (a *App) layoutAssetCard(gtx layout.Context, asset *media.Asset, index, total int) layout.Dimensions {
	widgets := a.widgetsFor(asset.ID)
	if widgets.preview.Clicked(gtx) {
		a.modal = asset
		a.modalStateSet = nil
		for index, mode := range []string{"auto", "6", "8"} {
			if mode == asset.FrameCountMode {
				a.modalFrameIndex = index
			}
		}
	}
	if widgets.remove.Clicked(gtx) {
		_ = a.engine.Store.Remove(a.session, asset.ID)
		return layout.Dimensions{}
	}
	if widgets.up.Clicked(gtx) && index > 0 {
		a.moveAsset(index, index-1)
	}
	if widgets.down.Clicked(gtx) && index < total-1 {
		a.moveAsset(index, index+1)
	}
	if widgets.analysis.Clicked(gtx) {
		_, _ = a.engine.Store.SetAnalysis(a.session, asset.ID, !asset.AnalysisRequested)
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bordered(gtx, 6, colorCard, colorBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widgets.preview.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.assetThumb(gtx, asset)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, 13, asset.Reference)
									l.Color = colorAccent
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, 12, assetMeta(asset))
									l.Color = colorTextDim
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if asset.Type == media.Audio {
										return bodyText(gtx, a.theme, "Declared only — the model does not hear audio")
									}
									if !asset.AnalysisRequested {
										l := material.Label(a.theme, 11, "Excluded from AI analysis")
										l.Color = colorDanger
										return l.Layout(gtx)
									}
									return layout.Dimensions{}
								}),
							)
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.iconButton(gtx, &widgets.up, "↑", index > 0)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.iconButton(gtx, &widgets.down, "↓", index < total-1)
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										label := "AI ✓"
										if !asset.AnalysisRequested {
											label = "AI ✗"
										}
										return a.iconButton(gtx, &widgets.analysis, label, true)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.iconButton(gtx, &widgets.remove, "✕", true)
									}),
								)
							}),
						)
					}),
				)
			})
		}),
	)
}

// moveAsset swaps two adjacent assets and renumbers.
func (a *App) moveAsset(from, to int) {
	assets := a.engine.Store.List(a.session)
	if from < 0 || to < 0 || from >= len(assets) || to >= len(assets) {
		return
	}
	ids := make([]string, len(assets))
	for index, asset := range assets {
		ids[index] = asset.ID
	}
	ids[from], ids[to] = ids[to], ids[from]
	_, _ = a.engine.Store.Reorder(a.session, ids)
}

// assetThumb renders the preview thumbnail or a type placeholder.
func (a *App) assetThumb(gtx layout.Context, asset *media.Asset) layout.Dimensions {
	size := gtx.Dp(56)
	gtx.Constraints.Min = image.Pt(size, size)
	gtx.Constraints.Max = image.Pt(size, size)
	if asset.PreviewPath != "" {
		if img := a.loadImage(asset.PreviewPath); img != nil {
			return widget.Image{Src: paint.NewImageOp(img), Fit: widget.Cover, Scale: 1}.Layout(gtx)
		}
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fill(gtx, 4, colorSurface)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				icon := "♪"
				if asset.Type == media.Video {
					icon = "▶"
				}
				l := material.Label(a.theme, 20, icon)
				l.Color = colorTextDim
				return l.Layout(gtx)
			})
		}),
	)
}

// loadImage decodes and caches a preview image.
func (a *App) loadImage(path string) image.Image {
	if img, ok := a.images[path]; ok {
		return img
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil
	}
	a.images[path] = img
	return img
}

// assetMeta renders the second line of an asset card.
func assetMeta(asset *media.Asset) string {
	meta := fmt.Sprintf("%s · %.1f MB", asset.Filename, float64(asset.Size)/(1024*1024))
	if asset.Duration > 0 {
		meta += fmt.Sprintf(" · %gs", asset.Duration)
	}
	return meta
}

// iconButton renders a tiny square button; disabled ones are dimmed.
func (a *App) iconButton(gtx layout.Context, btn *widget.Clickable, label string, enabled bool) layout.Dimensions {
	if !enabled {
		l := material.Label(a.theme, 12, label)
		l.Color = colorBorder
		return layout.UniformInset(4).Layout(gtx, l.Layout)
	}
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Label(a.theme, 12, label)
			l.Color = colorText
			return l.Layout(gtx)
		})
	})
}
