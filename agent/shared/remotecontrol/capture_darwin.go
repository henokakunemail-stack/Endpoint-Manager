//go:build darwin

package remotecontrol

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
)

type DarwinCapturer struct{}

func NewPlatformCapturer() ScreenCapturer {
	return &DarwinCapturer{}
}

func (c *DarwinCapturer) CaptureScreen() ([]byte, int, int, error) {
	width := 1280
	height := 720
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 30, G: 30, B: 30, A: 255}}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), width, height, nil
}

func (c *DarwinCapturer) InjectMouseEvent(action string, x, y int, button string, delta int) error {
	return nil
}

func (c *DarwinCapturer) InjectKeyboardEvent(action string, key string, code int) error {
	return nil
}
