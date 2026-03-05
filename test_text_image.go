package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func createTextImage(text string) ([]byte, error) {
	width := 800
	height := 800

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	col := color.RGBA{255, 255, 255, 255}

	lines := strings.Split(text, "\n")
	y := 50

	for _, line := range lines {
		point := fixed.Point26_6{X: fixed.I(20), Y: fixed.I(y)}
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(col),
			Face: basicfont.Face7x13,
			Dot:  point,
		}
		d.DrawString(line)
		y += 20 // line spacing
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func main() {
	b, _ := createTextImage("Rahasia!\nPromo XCLP diskon 50%\nHanya berlaku hari ini!")
	os.WriteFile("test_secret.png", b, 0644)
}
