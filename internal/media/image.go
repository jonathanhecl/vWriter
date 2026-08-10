package media

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/disintegration/imageorient"
	"golang.org/x/image/draw"

	// Register decoders for the accepted image formats.
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "image/png"
)

const (
	preparedMaxEdge = 1536
	previewMaxEdge  = 640
	jpegQualityHigh = 92
	jpegQualityLow  = 84
)

// processImage normalizes orientation and writes the prepared (model input)
// and preview (UI thumbnail) renditions of an image asset.
func processImage(asset *Asset) error {
	file, err := os.Open(asset.OriginalPath)
	if err != nil {
		return &Error{Code: "MEDIA_DECODE_FAILED", Message: "Could not read the image file.", Details: err.Error()}
	}
	defer file.Close()
	decoded, _, err := imageorient.Decode(file)
	if err != nil {
		return &Error{
			Code:    "MEDIA_DECODE_FAILED",
			Message: fmt.Sprintf("Could not decode image file: %v", err),
		}
	}
	bounds := decoded.Bounds()
	asset.Width, asset.Height = bounds.Dx(), bounds.Dy()

	asset.PreparedPath = filepath.Join(asset.artifactDir, "prepared.jpg")
	if err := saveResized(decoded, asset.PreparedPath, preparedMaxEdge, jpegQualityHigh); err != nil {
		return err
	}
	asset.PreviewPath = filepath.Join(asset.artifactDir, "preview.jpg")
	return saveResized(decoded, asset.PreviewPath, previewMaxEdge, jpegQualityLow)
}

// saveResized fits the image within a maxEdge square and saves it as JPEG.
func saveResized(source image.Image, dest string, maxEdge, quality int) error {
	if resized := fitWithin(source, maxEdge); resized != source {
		source = resized
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := jpeg.Encode(out, source, &jpeg.Options{Quality: quality}); err != nil {
		return &Error{Code: "MEDIA_DECODE_FAILED", Message: "Could not write the processed image.", Details: err.Error()}
	}
	return nil
}

// fitWithin returns the image scaled down so no edge exceeds maxEdge, or the
// original image when already small enough.
func fitWithin(source image.Image, maxEdge int) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxEdge && height <= maxEdge {
		return source
	}
	scale := float64(maxEdge) / float64(max(width, height))
	target := image.NewRGBA(image.Rect(0, 0, int(float64(width)*scale), int(float64(height)*scale)))
	draw.CatmullRom.Scale(target, target.Bounds(), source, bounds, draw.Over, nil)
	return target
}
