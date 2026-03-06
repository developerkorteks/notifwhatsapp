package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// CreateTextImage creates an elegant, gradient poster-like image for plain text.
func CreateTextImage(text string) ([]byte, error) {
	parsedFont, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    28,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	lines := strings.Split(text, "\n")
	lineHeight := 40
	paddingX := 40
	paddingY := 60
	width := 800

	height := (len(lines) * lineHeight) + (paddingY * 2)
	if height < 400 {
		height = 400
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw an elegant gradient background (Dark Navy to Slate)
	for y := 0; y < height; y++ {
		ratio := float64(y) / float64(height)
		r := uint8(23 - (ratio * (23 - 2)))
		g := uint8(37 - (ratio * (37 - 6)))
		b := uint8(84 - (ratio * (84 - 23)))

		c := color.RGBA{R: r, G: g, B: b, A: 255}
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}

	// Add a subtle border
	borderColor := color.RGBA{R: 56, G: 189, B: 248, A: 100}
	draw.Draw(img, image.Rect(10, 10, width-10, height-10), &image.Uniform{C: color.RGBA{R: 0, G: 0, B: 0, A: 0}}, image.Point{}, draw.Over)
	for y := 10; y < height-10; y++ {
		img.Set(10, y, borderColor)
		img.Set(width-10, y, borderColor)
	}
	for x := 10; x < width-10; x++ {
		img.Set(x, 10, borderColor)
		img.Set(x, height-10, borderColor)
	}

	textColor := color.RGBA{R: 248, G: 250, B: 252, A: 255}

	// Draw text
	startY := paddingY + 28
	for _, line := range lines {
		advance := font.MeasureString(face, line)
		lineWidth := advance.Round()
		startX := (width - lineWidth) / 2
		if startX < paddingX {
			startX = paddingX
		}

		point := fixed.Point26_6{X: fixed.I(startX), Y: fixed.I(startY)}
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(textColor),
			Face: face,
			Dot:  point,
		}

		d.DrawString(line)
		startY += lineHeight
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
