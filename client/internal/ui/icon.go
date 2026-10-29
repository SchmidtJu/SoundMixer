//go:build windows

package ui

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/lxn/walk"
)

func mixerIcon() (*walk.Icon, error) {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	background := color.RGBA{R: 23, G: 42, B: 69, A: 255}
	track := color.RGBA{R: 186, G: 204, B: 220, A: 255}
	knob := color.RGBA{R: 72, G: 199, B: 142, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)

	knobTops := []int{18, 10, 22, 6}
	for i := 0; i < 4; i++ {
		x := 5 + i*7
		for y := 4; y <= 27; y++ {
			img.SetRGBA(x, y, track)
			img.SetRGBA(x+1, y, track)
		}
		top := knobTops[i]
		for dy := 0; dy < 5; dy++ {
			for dx := -1; dx <= 2; dx++ {
				img.SetRGBA(x+dx, top+dy, knob)
			}
		}
	}
	return walk.NewIconFromImage(img)
}
