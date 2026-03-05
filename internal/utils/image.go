package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// CreateTextImage creates a black image with the provided white text on it.
func CreateTextImage(text string) ([]byte, error) {
	// Standard width, dynamic height based on lines
	width := 800
	lines := strings.Split(text, "\n")

	// approximate height: 50 top padding + (lines * 20 spacing) + 50 bottom padding
	height := 50 + (len(lines) * 20) + 50
	if height < 400 {
		height = 400
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Draw a stark black background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	// White Text
	col := color.RGBA{255, 255, 255, 255}

	y := 50

	for _, line := range lines {
		// Use fixed point math equivalent to integer pixel precision 20, y
		point := fixed.Point26_6{X: fixed.I(20), Y: fixed.I(y)}
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(col),
			Face: basicfont.Face7x13,
			Dot:  point,
		}
		d.DrawString(line)
		y += 20 // move down by 20 pixels per line
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
