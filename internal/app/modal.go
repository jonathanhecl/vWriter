package app

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/jonathanhecl/vWriter/internal/media"
)

// modalState holds the widgets of the media preview modal.
type modalState struct {
	close     widget.Clickable
	frameDrop dropdown
	endpoints widget.Bool
	resample  widget.Clickable
}

// layoutModal renders the open preview modal, if any.
func (a *App) layoutModal(gtx layout.Context) layout.Dimensions {
	if a.modal == nil {
		return layout.Dimensions{}
	}
	if a.modalStateSet == nil {
		a.modalStateSet = &modalState{endpoints: widget.Bool{}}
	}
	modal := a.modalStateSet

	asset := a.modal
	if modal.endpoints.Value != asset.IncludeEndpoints {
		modal.endpoints.Value = asset.IncludeEndpoints
	}
	frameOptions := []string{"Auto (8 frames)", "6 frames", "8 frames"}
	frameModes := []string{"auto", "6", "8"}

	if modal.close.Clicked(gtx) {
		a.modal = nil
		return layout.Dimensions{}
	}
	if modal.resample.Clicked(gtx) {
		endpoints := modal.endpoints.Value
		updated, err := a.engine.Store.Resample(a.session, asset.ID, frameModes[a.modalFrameIndex], &endpoints)
		if err != nil {
			a.toasts = append(a.toasts, toastMsg{text: errorText(err), isError: true})
		} else {
			delete(a.images, asset.PreviewPath)
			a.modal = updated
			asset = updated
		}
	}
	if modal.endpoints.Update(gtx) {
		// Applied on the next resample.
	}

	// Dimmed backdrop.
	paint.FillShape(gtx.Ops, color.NRGBA{A: 0xb0},
		clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		maxW := gtx.Dp(720)
		if gtx.Constraints.Max.X < maxW {
			maxW = gtx.Constraints.Max.X
		}
		gtx.Constraints.Max.X = maxW
		gtx.Constraints.Min.X = maxW
		gtx.Constraints.Max.Y = gtx.Dp(560)
		return card(gtx, 14, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, 15, fmt.Sprintf("%s — %s", asset.Reference, asset.Filename))
							l.Color = colorText
							return l.Layout(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.smallButton(gtx, &modal.close, "Close")
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutModalImage(gtx, asset)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if asset.Type != media.Video {
						return layout.Dimensions{}
					}
					return a.layoutVideoControls(gtx, asset, modal, frameOptions, frameModes)
				}),
			)
		})
	})
}

// layoutModalImage shows the contact sheet (video) or prepared image.
func (a *App) layoutModalImage(gtx layout.Context, asset *media.Asset) layout.Dimensions {
	path := asset.PreviewPath
	if asset.Type == media.Video && asset.ContactSheetPath != "" {
		path = asset.ContactSheetPath
	}
	img := a.loadImage(path)
	if img == nil {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return bodyText(gtx, a.theme, "No preview available for this file type.")
		})
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Image{Src: paint.NewImageOp(img), Fit: widget.Contain}.Layout(gtx)
	})
}

// layoutVideoControls renders the resample controls of a video preview.
func (a *App) layoutVideoControls(gtx layout.Context, asset *media.Asset, modal *modalState, frameOptions []string, frameModes []string) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return bodyText(gtx, a.theme, "What the model sees")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(150)
			chosen, changed := modal.frameDrop.Layout(gtx, a.theme, frameOptions, a.modalFrameIndex)
			if changed {
				a.modalFrameIndex = chosen
			}
			return layout.Dimensions{Size: image.Pt(gtx.Dp(150), gtx.Dp(34))}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				cb := material.CheckBox(a.theme, &modal.endpoints, "First & last")
				cb.Color = colorText
				cb.TextSize = 13
				return cb.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.smallButton(gtx, &modal.resample, "Resample")
			})
		}),
	)
}
