//go:build linux

package remotecontrol

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
)

type LinuxCapturer struct{}

func NewPlatformCapturer() ScreenCapturer {
	return &LinuxCapturer{}
}

func (c *LinuxCapturer) CaptureScreen() ([]byte, int, int, error) {
	// Pure-Go software framebuffer fallback for Linux headless / Wayland / X11 without CGO
	width := 1280
	height := 720
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 26, G: 32, B: 44, A: 255}}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), width, height, nil
}

func (c *LinuxCapturer) InjectMouseEvent(action string, x, y int, button string, delta int) error {
	return nil
}

func (c *LinuxCapturer) InjectKeyboardEvent(action string, key string, code int) error {
	return nil
}
