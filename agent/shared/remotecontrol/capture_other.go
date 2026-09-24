//go:build !windows && !linux && !darwin

package remotecontrol

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
)

type OtherCapturer struct{}

func NewPlatformCapturer() ScreenCapturer {
	return &OtherCapturer{}
}

func (c *OtherCapturer) CaptureScreen() ([]byte, int, int, error) {
	width := 1280
	height := 720
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 20, G: 20, B: 20, A: 255}}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), width, height, nil
}

func (c *OtherCapturer) InjectMouseEvent(action string, x, y int, button string, delta int) error {
	return nil
}

func (c *OtherCapturer) InjectKeyboardEvent(action string, key string, code int) error {
	return nil
}
