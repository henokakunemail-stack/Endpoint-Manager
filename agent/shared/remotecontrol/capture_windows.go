//go:build windows

package remotecontrol

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procSetCursorPos           = user32.NewProc("SetCursorPos")
	procMouseEvent             = user32.NewProc("mouse_event")
	procKeybdEvent             = user32.NewProc("keybd_event")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
)

const (
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1
	SRCCOPY     = 0x00CC0020
	BI_RGB      = 0

	MOUSEEVENTF_MOVE       = 0x0001
	MOUSEEVENTF_LEFTDOWN   = 0x0002
	MOUSEEVENTF_LEFTUP     = 0x0004
	MOUSEEVENTF_RIGHTDOWN  = 0x0008
	MOUSEEVENTF_RIGHTUP    = 0x0010
	MOUSEEVENTF_MIDDLEDOWN = 0x0020
	MOUSEEVENTF_MIDDLEUP   = 0x0040
	MOUSEEVENTF_WHEEL      = 0x0800

	KEYEVENTF_KEYUP = 0x0002
)

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type WindowsCapturer struct{}

func NewPlatformCapturer() ScreenCapturer {
	return &WindowsCapturer{}
}

func (c *WindowsCapturer) CaptureScreen() ([]byte, int, int, error) {
	w, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXSCREEN))
	h, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYSCREEN))
	width := int(w)
	height := int(h)
	if width <= 0 || height <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid screen dimensions: %dx%d", width, height)
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, 0, 0, fmt.Errorf("failed to get screen DC")
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, 0, 0, fmt.Errorf("failed to create compatible DC")
	}
	defer procDeleteDC.Call(hdcMem)

	hBitmap, _, _ := procCreateCompatibleBitmap.Call(hdcScreen, uintptr(width), uintptr(height))
	if hBitmap == 0 {
		return nil, 0, 0, fmt.Errorf("failed to create compatible bitmap")
	}
	defer procDeleteObject.Call(hBitmap)

	hOld, _, _ := procSelectObject.Call(hdcMem, hBitmap)
	defer procSelectObject.Call(hdcMem, hOld)

	ret, _, _ := procBitBlt.Call(hdcMem, 0, 0, uintptr(width), uintptr(height), hdcScreen, 0, 0, uintptr(SRCCOPY))
	if ret == 0 {
		return nil, 0, 0, fmt.Errorf("bitblt failed")
	}

	var bi bitmapInfoHeader
	bi.BiSize = uint32(unsafe.Sizeof(bi))
	bi.BiWidth = int32(width)
	bi.BiHeight = -int32(height) // Negative for top-down bitmap
	bi.BiPlanes = 1
	bi.BiBitCount = 32
	bi.BiCompression = BI_RGB

	bufSize := width * height * 4
	rawPixels := make([]byte, bufSize)

	ret, _, _ = procGetDIBits.Call(
		hdcMem,
		hBitmap,
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&rawPixels[0])),
		uintptr(unsafe.Pointer(&bi)),
		0,
	)
	if ret == 0 {
		return nil, 0, 0, fmt.Errorf("getdibits failed")
	}

	// Win32 DIB is BGRA, convert to RGBA in-place
	img := &image.RGBA{
		Pix:    rawPixels,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	for i := 0; i < bufSize; i += 4 {
		b := rawPixels[i]
		r := rawPixels[i+2]
		rawPixels[i] = r
		rawPixels[i+2] = b
		rawPixels[i+3] = 0xFF // opaque
	}

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 60}); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}

	return jpegBuf.Bytes(), width, height, nil
}

func (c *WindowsCapturer) InjectMouseEvent(action string, x, y int, button string, delta int) error {
	// Move cursor to absolute position
	if x >= 0 && y >= 0 {
		_, _, _ = procSetCursorPos.Call(uintptr(x), uintptr(y))
	}

	var flags uint32
	switch action {
	case "move":
		flags = MOUSEEVENTF_MOVE
	case "down":
		switch button {
		case "right":
			flags = MOUSEEVENTF_RIGHTDOWN
		case "middle":
			flags = MOUSEEVENTF_MIDDLEDOWN
		default:
			flags = MOUSEEVENTF_LEFTDOWN
		}
	case "up":
		switch button {
		case "right":
			flags = MOUSEEVENTF_RIGHTUP
		case "middle":
			flags = MOUSEEVENTF_MIDDLEUP
		default:
			flags = MOUSEEVENTF_LEFTUP
		}
	case "click":
		// Down + Up sequence
		c.InjectMouseEvent("down", x, y, button, 0)
		c.InjectMouseEvent("up", x, y, button, 0)
		return nil
	case "wheel":
		flags = MOUSEEVENTF_WHEEL
	}

	_, _, _ = procMouseEvent.Call(uintptr(flags), 0, 0, uintptr(delta), 0)
	return nil
}

func (c *WindowsCapturer) InjectKeyboardEvent(action string, key string, code int) error {
	if code <= 0 {
		return nil
	}
	var flags uint32
	if action == "up" {
		flags = KEYEVENTF_KEYUP
	}
	_, _, _ = procKeybdEvent.Call(uintptr(code), 0, uintptr(flags), 0)
	return nil
}
