package app

import (
	"image"
	"image/color"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/jonathanhecl/vWriter/internal/media"
)

// imageRoles are the selectable roles for image assets.
var imageRoles = []string{"person", "scene", "clothes", "accessory"}

// videoRoles are the selectable roles for video assets.
var videoRoles = []string{"reference", "movement", "camera"}

// audioRoles are the selectable roles for audio assets.
var audioRoles = []string{"music", "voice"}

// assetModalState holds all UI widget state for the asset role/label modal.
type assetModalState struct {
	asset    *media.Asset
	drag     gesture.Drag
	pos      image.Point // drag offset from modal center position (px)
	dragging bool

	roleDropdown   dropdown
	roleIndex      int // -1 = no role selected
	labelEditor    widget.Editor
	linkedDropdown dropdown // only for audio voice
	linkedIndex    int      // -1 = no link

	saveBtn  widget.Clickable
	closeBtn widget.Clickable
}

func newAssetModal(asset *media.Asset, allAssets []*media.Asset) *assetModalState {
	m := &assetModalState{
		asset:       asset,
		roleIndex:   -1,
		linkedIndex: -1,
	}
	m.labelEditor.SingleLine = true
	m.labelEditor.SetText(asset.Label)

	// Restore role selection.
	roles := rolesFor(asset.Type)
	for i, r := range roles {
		if r == asset.Role {
			m.roleIndex = i
			break
		}
	}

	// For audio voice, restore linked image.
	if asset.Type == media.Audio && asset.LinkedAssetID != "" {
		imgs := imageAssetsFrom(allAssets)
		for i, img := range imgs {
			if img.ID == asset.LinkedAssetID {
				m.linkedIndex = i
				break
			}
		}
	}
	return m
}

// rolesFor returns the role options for a given asset type.
func rolesFor(t media.AssetType) []string {
	switch t {
	case media.Image:
		return imageRoles
	case media.Video:
		return videoRoles
	case media.Audio:
		return audioRoles
	}
	return nil
}

// imageAssetsFrom filters and returns only image assets from the list.
func imageAssetsFrom(assets []*media.Asset) []*media.Asset {
	var imgs []*media.Asset
	for _, a := range assets {
		if a.Type == media.Image {
			imgs = append(imgs, a)
		}
	}
	return imgs
}

// layoutAssetRoleModal renders the floating draggable asset role modal.
// Must be called at the top of the main layout stack (overlay).
func (a *App) layoutAssetRoleModal(gtx layout.Context) layout.Dimensions {
	m := a.assetModal
	if m == nil {
		return layout.Dimensions{}
	}
	asset := m.asset
	allAssets := a.engine.Store.List(a.session)

	// Handle button clicks.
	if m.closeBtn.Clicked(gtx) {
		a.assetModal = nil
		return layout.Dimensions{}
	}
	if m.saveBtn.Clicked(gtx) {
		roles := rolesFor(asset.Type)
		role := ""
		if m.roleIndex >= 0 && m.roleIndex < len(roles) {
			role = roles[m.roleIndex]
		}
		label := m.labelEditor.Text()
		linkedID := ""
		if asset.Type == media.Audio && role == "voice" {
			imgs := imageAssetsFrom(allAssets)
			if m.linkedIndex >= 0 && m.linkedIndex < len(imgs) {
				linkedID = imgs[m.linkedIndex].ID
			}
		}
		_, _ = a.engine.Store.SetRole(a.session, asset.ID, role, label, linkedID)
		a.autoSaveCurrentPreset()
		a.assetModal = nil
		a.window.Invalidate()
		return layout.Dimensions{}
	}

	// Handle drag events for modal movement.
	for {
		ev, ok := m.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch ev.Kind {
		case pointer.Press:
			m.dragging = true
		case pointer.Drag:
			m.pos.X += int(ev.Position.X)
			m.pos.Y += int(ev.Position.Y)
			a.window.Invalidate()
		case pointer.Release, pointer.Cancel:
			m.dragging = false
		}
	}

	// Full-screen overlay.
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fill(gtx, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 0x90})
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			maxW := gtx.Constraints.Max.X
			maxH := gtx.Constraints.Max.Y
			modalW := gtx.Dp(360)

			cx := (maxW-modalW)/2 + m.pos.X
			cy := maxH/5 + m.pos.Y

			if cx < 8 {
				cx = 8
			}
			if cy < 8 {
				cy = 8
			}
			if cx+modalW > maxW-8 {
				cx = maxW - 8 - modalW
			}

			defer op.Offset(image.Pt(cx, cy)).Push(gtx.Ops).Pop()
			gtx.Constraints.Min = image.Pt(modalW, 0)
			gtx.Constraints.Max = image.Pt(modalW, maxH-cy-8)

			return a.layoutAssetRoleModalContent(gtx, m, asset, allAssets)
		}),
	)
}

func (a *App) layoutAssetRoleModalContent(gtx layout.Context, m *assetModalState, asset *media.Asset, allAssets []*media.Asset) layout.Dimensions {
	roles := rolesFor(asset.Type)
	roleNamesWithNone := append([]string{"(no role)"}, roles...)
	roleIdxForDropdown := m.roleIndex + 1

	imgs := imageAssetsFrom(allAssets)
	imgNames := make([]string, len(imgs))
	for i, img := range imgs {
		imgNames[i] = img.Reference + " " + img.Filename
	}
	imgNamesWithNone := append([]string{"(none)"}, imgNames...)
	linkedIdxForDropdown := m.linkedIndex + 1

	return card(gtx, 18, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,

			// Drag handle / title row
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							m.drag.Add(gtx.Ops)
							l := material.Label(a.theme, 15, asset.Reference+" — Set type")
							l.Color = colorText
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return m.closeBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, 14, "✕")
								l.Color = colorTextDim
								return layout.UniformInset(4).Layout(gtx, l.Layout)
							})
						}),
					)
				})
			}),

			// Role selector label
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 12, "Type")
					l.Color = colorTextDim
					return l.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					chosen, changed := m.roleDropdown.Layout(gtx, a.theme, roleNamesWithNone, roleIdxForDropdown)
					if changed {
						m.roleIndex = chosen - 1
					}
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(34))}
				})
			}),

			// Label input
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, 12, "Label (optional)")
					l.Color = colorTextDim
					return l.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.singlelineBox(gtx, &m.labelEditor, "e.g. John, office background…")
				})
			}),

			// Linked image (audio voice only)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				selectedRole := ""
				if m.roleIndex >= 0 && m.roleIndex < len(roles) {
					selectedRole = roles[m.roleIndex]
				}
				if asset.Type != media.Audio || selectedRole != "voice" || len(imgs) == 0 {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, 12, "Voice of image (optional)")
							l.Color = colorTextDim
							return l.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							chosen, changed := m.linkedDropdown.Layout(gtx, a.theme, imgNamesWithNone, linkedIdxForDropdown)
							if changed {
								m.linkedIndex = chosen - 1
							}
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(34))}
						})
					}),
				)
			}),

			// Save / close row
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.smallButton(gtx, &m.saveBtn, "Apply")
					}),
				)
			}),
		)
	})
}
