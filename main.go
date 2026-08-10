// vWriter is a native desktop app that turns a creative brief plus
// image/video/audio references into a structured full-reference video prompt
// using a local Ollama vision model.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/jonathanhecl/vWriter/internal/ollama"
)

func main() {
	debug := flag.Bool("debug", false, "list Ollama models with vision capability and exit")
	ollamaURL := flag.String("url", ollama.DefaultURL, "Ollama server URL")
	flag.Parse()
	if *debug {
		debugListModels(*ollamaURL)
		return
	}
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

// debugListModels prints the installed Ollama models and their vision
// capability, then exits. Temporary verification aid until the UI lands.
func debugListModels(rawURL string) {
	client, err := ollama.NewClient(rawURL)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	version, err := client.Version(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Ollama %s at %s\n\n", version, client.BaseURL())
	models, err := client.Tags(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(models) == 0 {
		fmt.Println("No models installed. Use `ollama pull <name>` first.")
		return
	}
	for _, model := range models {
		vision := "no "
		if info, err := client.Show(ctx, model.Name); err == nil && info.HasVision() {
			vision = "yes"
		}
		fmt.Printf("%-30s %-8s vision: %s\n", model.Name, model.Details.ParameterSize, vision)
	}
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
