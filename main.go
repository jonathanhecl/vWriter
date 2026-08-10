// vWriter is a native desktop app that turns a creative brief plus
// image/video/audio references into a structured full-reference video prompt
// using a local Ollama vision model.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	giouiApp "gioui.org/app"
	"gioui.org/unit"

	uiApp "github.com/jonathanhecl/vWriter/internal/app"
	"github.com/jonathanhecl/vWriter/internal/ollama"
)

func main() {
	giouiApp.ID = "com.github.jonathanhecl.vwriter"
	debug := flag.Bool("debug", false, "list Ollama models with vision capability and exit")
	ollamaURL := flag.String("url", ollama.DefaultURL, "Ollama server URL")
	flag.Parse()
	if *debug {
		debugListModels(*ollamaURL)
		return
	}
	go func() {
		window := new(giouiApp.Window)
		window.Option(giouiApp.Title("vWriter"), giouiApp.Size(unit.Dp(1240), unit.Dp(780)))
		if err := uiApp.Run(window); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	giouiApp.Main()
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
