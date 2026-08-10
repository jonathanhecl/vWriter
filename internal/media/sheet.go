package media

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"os"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	xdraw "golang.org/x/image/draw"
)

const (
	sheetCellWidth = 384
	sheetGutter    = 28
	sheetMinHeight = 216
	sheetMaxHeight = 512
)

var (
	sheetBackground = color.NRGBA{R: 0x10, G: 0x12, B: 0x16, A: 0xff}
	sheetLabelColor = color.NRGBA{R: 0xb7, G: 0xbc, B: 0xc6, A: 0xff}
)

// buildContactSheet renders the sampled frames as one ordered, numbered grid:
// the exact visual representation the model receives for a video reference.
// Cells read left-to-right, then top-to-bottom.
func buildContactSheet(frames []Frame, dest string) error {
	columns := 3
	if len(frames) > 6 {
		columns = 4
	}
	rows := int(math.Ceil(float64(len(frames)) / float64(columns)))

	first, err := decodeImage(frames[0].Path)
	if err != nil {
		return err
	}
	firstBounds := first.Bounds()
	aspect := float64(firstBounds.Dy()) / math.Max(float64(firstBounds.Dx()), 1)
	cellHeight := int(math.Round(sheetCellWidth * aspect))
	cellHeight = max(sheetMinHeight, min(sheetMaxHeight, cellHeight))
	cellTotal := cellHeight + sheetGutter

	sheet := image.NewRGBA(image.Rect(0, 0, columns*sheetCellWidth, rows*cellTotal))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(sheetBackground), image.Point{}, draw.Src)

	for index, frame := range frames {
		source, err := decodeImage(frame.Path)
		if err != nil {
			return err
		}
		fitted := contain(source, sheetCellWidth, cellHeight)
		column, row := index%columns, index/columns
		left := column*sheetCellWidth + (sheetCellWidth-fitted.Bounds().Dx())/2
		top := row*cellTotal + sheetGutter + (cellHeight-fitted.Bounds().Dy())/2
		draw.Draw(sheet, fitted.Bounds().Add(image.Pt(left, top)), fitted, fitted.Bounds().Min, draw.Src)
		writeLabel(sheet, strconv.Itoa(index+1), column*sheetCellWidth+10, row*cellTotal+sheetGutter-8)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := jpeg.Encode(out, sheet, &jpeg.Options{Quality: 90}); err != nil {
		return &Error{Code: "MEDIA_DECODE_FAILED", Message: "Could not write the contact sheet.", Details: err.Error()}
	}
	return nil
}

// decodeImage loads a JPEG or PNG frame from disk.
func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		return nil, &Error{Code: "MEDIA_DECODE_FAILED", Message: "Could not decode a sampled frame.", Details: err.Error()}
	}
	return decoded, nil
}

// contain scales an image down to fit within maxWidth x maxHeight.
func contain(source image.Image, maxWidth, maxHeight int) image.Image {
	bounds := source.Bounds()
	scale := math.Min(float64(maxWidth)/float64(bounds.Dx()), float64(maxHeight)/float64(bounds.Dy()))
	if scale >= 1 {
		return source
	}
	target := image.NewRGBA(image.Rect(0, 0, int(float64(bounds.Dx())*scale), int(float64(bounds.Dy())*scale)))
	xdraw.CatmullRom.Scale(target, target.Bounds(), source, bounds, xdraw.Over, nil)
	return target
}

// writeLabel draws the 1-based cell number into the gutter above a cell.
func writeLabel(sheet *image.RGBA, text string, x, baseline int) {
	drawer := &font.Drawer{
		Dst:  sheet,
		Src:  image.NewUniform(sheetLabelColor),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, baseline),
	}
	drawer.DrawString(text)
}
