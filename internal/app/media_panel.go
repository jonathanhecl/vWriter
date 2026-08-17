package app

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
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

// mediaDragEngageDp is the horizontal travel a press must exceed before the
// row starts dragging. It is far larger than the platform touch slop so that
// ordinary clicks (which always carry a few pixels of jitter) reach the card
// buttons instead of being stolen by the scroll gesture.
const mediaDragEngageDp = 24

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

// mediaScrollLimit returns the maximum index the media row can start at
// while still showing a full viewport of items.
func (a *App) mediaScrollLimit(totalItems int) int {
	visibleItems := a.mediaList.Position.Count
	if visibleItems < 1 {
		visibleItems = 3
	}
	return max(totalItems-visibleItems, 0)
}

// scrollMediaBy scrolls the media row by delta items, pinning the target item
// to the left edge (Offset 0). This keeps the row item-aligned so the first
// card is always reachable: a leftover pixel offset from wheel or drag would
// otherwise make the layout re-advance First and clip the start.
func (a *App) scrollMediaBy(delta int) {
	if a.mediaTotalItems < 1 {
		a.mediaTotalItems = 1
	}
	first := a.mediaList.Position.First + delta
	if first < 0 {
		first = 0
	} else if first >= a.mediaTotalItems {
		first = a.mediaTotalItems - 1
	}
	if first != a.mediaList.Position.First || a.mediaList.Position.Offset != 0 {
		a.mediaList.Position.First = first
		a.mediaList.Position.Offset = 0
		a.mediaList.Position.BeforeEnd = true
		if a.window != nil {
			a.window.Invalidate()
		}
	}
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

	var filteredAssets []*media.Asset
	for _, asset := range assets {
		if filter == "all" || (filter == "image" && asset.Type == media.Image) ||
			(filter == "video" && asset.Type == media.Video) || (filter == "audio" && asset.Type == media.Audio) {
			filteredAssets = append(filteredAssets, asset)
		}
	}
	totalItems := len(filteredAssets) + 1
	a.mediaTotalItems = totalItems
	if a.mediaList.Position.First >= totalItems {
		a.mediaList.Position.First = max(totalItems-1, 0)
		a.mediaList.Position.Offset = 0
		a.mediaList.Position.BeforeEnd = true
	}

	return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Header Top Row (Line 1: MEDIA label left, Clear button right)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return sectionLabel(gtx, a.theme, "MEDIA")
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(assets) == 0 {
							return layout.Dimensions{}
						}
						return a.smallButton(gtx, &a.clearMediaBtn, "Clear")
					}),
				)
			}),
			// Main Title (Line 2: Images, video & audio)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 16, "Images, video & audio")
					l.Color = colorText
					return l.Layout(gtx)
				})
			}),
			// Subtitle
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 12, "Add up to 9 images, 3 videos and 3 audio files.")
					l.Color = colorTextDim
					return l.Layout(gtx)
				})
			}),
			// Filter Pills Row (Single line horizontal list)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							a.filterList.Axis = layout.Horizontal
							pillsGtx := gtx
							pillsGtx.Constraints.Min.Y = gtx.Dp(28)
							pillsGtx.Constraints.Max.Y = pillsGtx.Constraints.Min.Y

							return a.filterList.Layout(pillsGtx, 4, func(gtx layout.Context, index int) layout.Dimensions {
								return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									switch index {
									case 0:
										return a.filterPill(gtx, &a.filterAllBtn, fmt.Sprintf("All %d/12", len(assets)), filter == "all")
									case 1:
										return a.filterPill(gtx, &a.filterImgBtn, fmt.Sprintf("🖼 Images %d/9", imgCount), filter == "image")
									case 2:
										return a.filterPill(gtx, &a.filterVidBtn, fmt.Sprintf("🎬 Video %d/3", vidCount), filter == "video")
									case 3:
										return a.filterPill(gtx, &a.filterAudBtn, fmt.Sprintf("🎵 Audio %d/3", audCount), filter == "audio")
									}
									return layout.Dimensions{}
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.iconButton(gtx, &a.scrollLeftBtn, "‹",
											a.mediaList.Position.First > 0 || a.mediaList.Position.Offset != 0)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.iconButton(gtx, &a.scrollRightBtn, "›",
											a.mediaList.Position.First < a.mediaScrollLimit(totalItems))
									}),
								)
							})
						}),
					)
				})
			}),
			// Asset Grid / Horizontal Card Row with Interactive Scrollbar
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				a.mediaList.Axis = layout.Horizontal

				hGtx := gtx
				cardHeight := gtx.Dp(135)
				hGtx.Constraints.Min.Y = cardHeight
				hGtx.Constraints.Max.Y = cardHeight

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims := a.mediaList.Layout(hGtx, totalItems, func(gtx layout.Context, index int) layout.Dimensions {
							if index < len(filteredAssets) {
								asset := filteredAssets[index]
								return layout.Inset{Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.layoutAssetCard(gtx, asset, index, len(filteredAssets))
								})
							}
							return a.layoutAddMediaCard(gtx)
						})

						return dims
					}),
					// Interactive Horizontal Scrollbar Track & Thumb
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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

							for {
								event, ok := gtx.Event(pointer.Filter{
									Target: &a.scrollbarClickable,
									Kinds:  pointer.Press,
								})
								if !ok {
									break
								}
								if ev, ok := event.(pointer.Event); ok {
									if ev.Position.X < float32(thumbLeft) {
										a.scrollMediaBy(-1)
									} else if ev.Position.X > float32(thumbLeft+thumbWidth) {
										a.scrollMediaBy(1)
									}
								}
							}

							return a.scrollbarClickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								// Draw track background
								bgGtx := gtx
								bgGtx.Constraints.Min = image.Pt(trackWidth, gtx.Dp(8))
								bgGtx.Constraints.Max = bgGtx.Constraints.Min
								fill(bgGtx, 4, colorCard)

								// Draw scrollbar thumb
								thumbOff := op.Offset(image.Pt(thumbLeft, 0)).Push(gtx.Ops)
								thumbGtx := gtx
								thumbGtx.Constraints.Min = image.Pt(thumbWidth, gtx.Dp(8))
								thumbGtx.Constraints.Max = thumbGtx.Constraints.Min
								fill(thumbGtx, 4, colorAccent)
								thumbOff.Pop()

								return layout.Dimensions{Size: bgGtx.Constraints.Min}
							})
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
	if !a.dbgCardLaidOut {
		a.dbgCardLaidOut = true
		dbgLog("layoutAddMediaCard: laid out")
	}
	event.Op(gtx.Ops, &a.dbgCardProbe)
	for {
		p, ok := gtx.Event(pointer.Filter{
			Target: &a.dbgCardProbe,
			Kinds:  pointer.Press | pointer.Release | pointer.Enter | pointer.Leave | pointer.Cancel | pointer.Drag,
		})
		if !ok {
			break
		}
		ev, ok := p.(pointer.Event)
		if !ok {
			continue
		}
		dbgLog(fmt.Sprintf("cardProbe kind=%v pos=%v", ev.Kind, ev.Position))
	}
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
		// Open role/label modal.
		a.assetModal = newAssetModal(asset, a.engine.Store.List(a.session))
	}
	if widgets.remove.Clicked(gtx) {
		_ = a.engine.Store.Remove(a.session, asset.ID)
		a.autoSaveCurrentPreset()
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
										l := material.Label(a.theme, 12, asset.Reference)
										l.Color = colorText
										l.MaxLines = 1
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										// Show frame-anchor type or role badge.
										badgeText := asset.Role
										badgeColored := asset.Role != ""
										switch asset.Role {
										case media.RoleFirstFrame:
											badgeText = "FIRST FRAME"
										case media.RoleLastFrame:
											badgeText = "LAST FRAME"
										}
										if asset.Role == media.RoleFirstFrame || asset.Role == media.RoleLastFrame {
											badgeColored = true
										} else if badgeText == "" {
											badgeText = asset.Filename
											if len(badgeText) > 14 {
												badgeText = badgeText[:12] + "…"
											}
										} else if asset.Label != "" {
											badgeText = asset.Role + ": " + asset.Label
											if len(badgeText) > 16 {
												badgeText = badgeText[:14] + "…"
											}
										}
										l := material.Label(a.theme, 10, badgeText)
										if badgeColored {
											l.Color = colorAccent
										} else {
											l.Color = colorTextDim
										}
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
