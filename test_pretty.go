package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func createPrettyImage(text string) ([]byte, error) {
	// Parse the font
	parsedFont, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}

	parsedBoldFont, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	_ = parsedBoldFont // for future use

	// Create a font face
	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    28,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	// Measure text to determine dynamic height
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

	// Draw an elegant gradient background (Dark slate to darker slate)
	for y := 0; y < height; y++ {
		ratio := float64(y) / float64(height)
		// Top color: rgb(23, 37, 84) - Tailwind blue-950
		// Bottom color: rgb(2, 6, 23) - Tailwind slate-950
		r := uint8(23 - (ratio * (23 - 2)))
		g := uint8(37 - (ratio * (37 - 6)))
		b := uint8(84 - (ratio * (84 - 23)))

		c := color.RGBA{r, g, b, 255}
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}

	// Add a subtle border
	borderColor := color.RGBA{56, 189, 248, 100} // tailwind sky-400 with alpha
	draw.Draw(img, image.Rect(10, 10, width-10, height-10), &image.Uniform{color.RGBA{0, 0, 0, 0}}, image.Point{}, draw.Over)
	for y := 10; y < height-10; y++ {
		img.Set(10, y, borderColor)
		img.Set(width-10, y, borderColor)
	}
	for x := 10; x < width-10; x++ {
		img.Set(x, 10, borderColor)
		img.Set(x, height-10, borderColor)
	}

	textColor := color.RGBA{248, 250, 252, 255} // tailwind slate-50

	// Draw text
	startY := paddingY + 28 // baseline of the first line
	for _, line := range lines {
		// Calculate the exact width of the line to center it
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

func main() {
	text := "Toko ICS STORE\n\nPromo Spesial XL Akrab VIP!\nDapatkan Diskon 50% untuk Pembelian Pertama\nBerlaku Sampai Besok.\n\nWebsite: https://ics-store.my.id\nWhatsApp: +6285951475620"
	b, err := createPrettyImage(text)
	if err != nil {
		panic(err)
	}
	os.WriteFile("test_pretty.png", b, 0644)
	fmt.Println("Done: test_pretty.png created")
}
