// vWriter is a native desktop app that turns a creative brief plus
// image/video/audio references into a structured full-reference video prompt
// using a local Ollama vision model.
package main

import (
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func main() {
	go func() {
		window := new(app.Window)
		window.Option(app.Title("vWriter"), app.Size(unit.Dp(1100), unit.Dp(720)))
		if err := run(window); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	theme := material.NewTheme()
	var ops op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.H5(theme, "vWriter — scaffold ready")
				label.Color = color.NRGBA{R: 200, G: 205, B: 215, A: 255}
				return label.Layout(gtx)
			})
			event.Frame(gtx.Ops)
		}
	}
}
