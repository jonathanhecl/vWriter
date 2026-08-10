package app

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"gioui.org/layout"
	"gioui.org/op"
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

	imgCount, vidCount, audCount := 0, 0, 0
	for _, asset := range assets {
		switch asset.Type {
		case media.Image:
			imgCount++
		case media.Video:
			vidCount++
		case media.Audio:
			audCount++
		}
	}

	filter := a.mediaFilter
	if filter == "" {
		filter = "all"
	}

	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Header Top Row with Clear pinned to Top-Right (layout.NE)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return sectionLabel(gtx, a.theme, "MEDIA")
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, 16, "Images, video & audio")
									l.Color = colorText
									return l.Layout(gtx)
								})
							}),
						)
					}),
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						if len(assets) == 0 {
							return layout.Dimensions{}
						}
						return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.smallButton(gtx, &a.clearMediaBtn, "Clear")
						})
					}),
				)
			}),
			// Subtitle
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 12, "Add up to 9 images, 3 videos and 3 audio files.")
					l.Color = colorTextDim
					return l.Layout(gtx)
				})
			}),
			// Filter Pills Row with Scroll Arrows
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.filterPill(gtx, &a.filterAllBtn, fmt.Sprintf("All %d/12", len(assets)), filter == "all")
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.filterPill(gtx, &a.filterImgBtn, fmt.Sprintf("🖼 Images %d/9", imgCount), filter == "image")
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.filterPill(gtx, &a.filterVidBtn, fmt.Sprintf("🎬 Video %d/3", vidCount), filter == "video")
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.filterPill(gtx, &a.filterAudBtn, fmt.Sprintf("🎵 Audio %d/3", audCount), filter == "audio")
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.iconButton(gtx, &a.scrollLeftBtn, "‹", a.mediaList.Position.First > 0)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.iconButton(gtx, &a.scrollRightBtn, "›", true)
								}),
							)
						}),
					)
				})
			}),
			// Asset Grid / Horizontal Card Row with Visible Scrollbar
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var filteredAssets []*media.Asset
				for _, asset := range assets {
					switch filter {
					case "image":
						if asset.Type == media.Image {
							filteredAssets = append(filteredAssets, asset)
						}
					case "video":
						if asset.Type == media.Video {
							filteredAssets = append(filteredAssets, asset)
						}
					case "audio":
						if asset.Type == media.Audio {
							filteredAssets = append(filteredAssets, asset)
						}
					default:
						filteredAssets = append(filteredAssets, asset)
					}
				}

				totalItems := len(filteredAssets) + 1
				a.mediaList.Axis = layout.Horizontal

				hGtx := gtx
				cardHeight := gtx.Dp(135)
				hGtx.Constraints.Min.Y = cardHeight
				hGtx.Constraints.Max.Y = cardHeight

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.mediaList.Layout(hGtx, totalItems, func(gtx layout.Context, index int) layout.Dimensions {
							if index < len(filteredAssets) {
								asset := filteredAssets[index]
								return layout.Inset{Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.layoutAssetCard(gtx, asset, index, len(filteredAssets))
								})
							}
							return a.layoutAddMediaCard(gtx)
						})
					}),
					// Visible Horizontal Scrollbar Indicator
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							total := float32(totalItems)
							first := float32(a.mediaList.Position.First)
							count := float32(a.mediaList.Position.Count)
							if count == 0 {
								count = 3
							}
							leftRatio := first / total
							widthRatio := count / total
							if widthRatio >= 1.0 {
								return layout.Dimensions{}
							}

							trackWidth := gtx.Constraints.Max.X
							if trackWidth <= 0 {
								return layout.Dimensions{}
							}
							thumbLeft := int(leftRatio * float32(trackWidth))
							thumbWidth := max(int(widthRatio*float32(trackWidth)), gtx.Dp(30))
							if thumbLeft+thumbWidth > trackWidth {
								thumbLeft = trackWidth - thumbWidth
							}

							// Draw track background
							bgGtx := gtx
							bgGtx.Constraints.Min = image.Pt(trackWidth, gtx.Dp(4))
							bgGtx.Constraints.Max = bgGtx.Constraints.Min
							fill(bgGtx, 2, colorSurface)

							// Draw scrollbar thumb
							thumbOff := op.Offset(image.Pt(thumbLeft, 0)).Push(gtx.Ops)
							thumbGtx := gtx
							thumbGtx.Constraints.Min = image.Pt(thumbWidth, gtx.Dp(4))
							thumbGtx.Constraints.Max = thumbGtx.Constraints.Min
							fill(thumbGtx, 2, colorTextDim)
							thumbOff.Pop()

							return layout.Dimensions{Size: bgGtx.Constraints.Min}
						})
					}),
				)
			}),
		)
	})
}

func (a *App) filterPill(gtx layout.Context, btn *widget.Clickable, label string, active bool) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bgColor := colorSurface
		textColor := colorTextDim
		if active {
			bgColor = colorCard
			textColor = colorText
		}
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				bordered(gtx, 14, bgColor, colorBorder)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 12, label)
					l.Color = textColor
					return l.Layout(gtx)
				})
			},
		)
	})
}

func (a *App) layoutAddMediaCard(gtx layout.Context) layout.Dimensions {
	cardWidth := gtx.Dp(145)
	cardHeight := gtx.Dp(135)
	gtx.Constraints.Min = image.Pt(cardWidth, cardHeight)
	gtx.Constraints.Max = gtx.Constraints.Min

	return a.addMediaCardBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				bordered(gtx, 8, colorSurface, colorBorder)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, 20, "+")
							l.Color = colorTextDim
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, 12, "Add media")
							l.Color = colorText
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, 10, "Drop files here")
							l.Color = colorTextDim
							return l.Layout(gtx)
						}),
					)
				})
			},
		)
	})
}

// layoutAssetCard renders one asset in card mode.
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

	cardWidth := gtx.Dp(145)
	gtx.Constraints.Min.X = cardWidth
	gtx.Constraints.Max.X = cardWidth

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bordered(gtx, 8, colorCard, colorBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				// Thumbnail top
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widgets.preview.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{}.Layout(gtx,
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								return a.assetThumbCard(gtx, asset, cardWidth, gtx.Dp(85))
							}),
							layout.Expanded(func(gtx layout.Context) layout.Dimensions {
								if asset.Type == media.Video && asset.Duration > 0 {
									return layout.Inset{Top: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layout.Background{}.Layout(gtx,
												func(gtx layout.Context) layout.Dimensions {
													fill(gtx, 4, colorBackground)
													return layout.Dimensions{Size: gtx.Constraints.Min}
												},
												func(gtx layout.Context) layout.Dimensions {
													return layout.Inset{Top: 2, Bottom: 2, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
														mins := int(asset.Duration) / 60
														secs := int(asset.Duration) % 60
														l := material.Label(a.theme, 10, fmt.Sprintf("%02d:%02d", mins, secs))
														l.Color = colorText
														return l.Layout(gtx)
													})
												},
											)
										})
									})
								}
								return layout.Dimensions{}
							}),
						)
					})
				}),
				// Bottom Footer Bar
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, 12, fmt.Sprintf("<%s>", asset.Reference))
										l.Color = colorText
										l.MaxLines = 1
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										sub := asset.ID
										if len(sub) > 12 {
											sub = sub[:12] + "..."
										}
										l := material.Label(a.theme, 10, sub)
										l.Color = colorTextDim
										l.MaxLines = 1
										return l.Layout(gtx)
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.iconButton(gtx, &widgets.remove, "✕", true)
							}),
						)
					})
				}),
			)
		}),
	)
}

// assetThumbCard renders the preview thumbnail for cards.
func (a *App) assetThumbCard(gtx layout.Context, asset *media.Asset, width, height int) layout.Dimensions {
	gtx.Constraints.Min = image.Pt(width, height)
	gtx.Constraints.Max = image.Pt(width, height)
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
				icon := "🖼"
				if asset.Type == media.Video {
					icon = "🎬"
				} else if asset.Type == media.Audio {
					icon = "🎵"
				}
				l := material.Label(a.theme, 22, icon)
				l.Color = colorTextDim
				return l.Layout(gtx)
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
