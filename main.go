package main

import (
	"fmt"
	"log"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC        = user32.NewProc("GetDC")
	procReleaseDC    = user32.NewProc("ReleaseDC")
	procGetPixel     = gdi32.NewProc("GetPixel")
	procSendInput    = user32.NewProc("SendInput")
	procSetCursorPos = user32.NewProc("SetCursorPos")
)

const (
	INPUT_MOUSE          = 0
	MOUSEEVENTF_LEFTDOWN = 0x0002
	MOUSEEVENTF_LEFTUP   = 0x0004
	checkRadius          = 3
	tolerance            = 5  // Допуск для каждого цветового канала
	screenCheckMS        = 10 // Интервал проверки
)

type MOUSEINPUT struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type INPUT struct {
	Type uint32
	Mi   MOUSEINPUT
	_    uint32 // Выравнивание
}

var targetColors = []struct{ R, G, B uint8 }{
	{R: 0xfe, G: 0x00, B: 0xfe}, // #fe00fe
	{R: 0xfb, G: 0x00, B: 0xf9}, // #fb00f9
	{R: 0xfc, G: 0x00, B: 0xfb}, // #fc00fb
	{R: 0xfb, G: 0x00, B: 0xfa}, // #fb00fa
}

func main() {
	hdc := getDC(0)
	defer releaseDC(0, hdc)

	width := getSystemMetrics(0)
	height := getSystemMetrics(1)
	centerX, centerY := width/2, height/2

	fmt.Println("Triggerbot started! Target colors:")
	for _, c := range targetColors {
		fmt.Printf("#%02X%02X%02X\n", c.R, c.G, c.B)
	}

	for {
		if checkColors(hdc, centerX, centerY) {
			clickLeftMouse()
			clickAt(centerX, centerY)
		}
		time.Sleep(screenCheckMS * time.Millisecond)
	}
}

func clickAt(x, y int) {
	// Устанавливаем позицию курсора
	_, _, err := procSetCursorPos.Call(
		uintptr(x),
		uintptr(y),
	)
	if err != nil && err.Error() != "The operation completed successfully." {
		log.Printf("SetCursorPos error: %v", err)
	}

	// Создаем события нажатия и отпускания ЛКМ
	downInput := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			DwFlags:     MOUSEEVENTF_LEFTDOWN,
			Time:        uint32(time.Now().UnixNano() / 1e6),
			DwExtraInfo: 0,
		},
	}

	upInput := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			DwFlags:     MOUSEEVENTF_LEFTUP,
			Time:        uint32(time.Now().UnixNano() / 1e6),
			DwExtraInfo: 0,
		},
	}

	// Отправляем события
	if success := SendInput([]INPUT{downInput, upInput}); success != 2 {
		log.Printf("Failed to send input, sent %d events", success)
	}
}

func SendInput(inputs []INPUT) uint32 {
	if len(inputs) == 0 {
		return 0
	}

	// Важно: использовать правильный размер структуры
	size := unsafe.Sizeof(INPUT{})
	ret, _, err := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(size),
	)

	if err != nil && err.Error() != "The operation completed successfully." {
		log.Printf("SendInput error: %v", err)
	}

	return uint32(ret)
}

func checkColors(hdc uintptr, x, y int) bool {
	for dx := -checkRadius; dx <= checkRadius; dx++ {
		for dy := -checkRadius; dy <= checkRadius; dy++ {
			color := getPixel(hdc, x+dx, y+dy)
			r, g, b := extractRGB(color)

			for _, tc := range targetColors {
				if matchColor(r, g, b, tc.R, tc.G, tc.B) {
					return true
				}
			}
		}
	}
	return false
}

func matchColor(r, g, b, tr, tg, tb uint8) bool {
	return withinTolerance(r, tr) &&
		withinTolerance(g, tg) &&
		withinTolerance(b, tb)
}

func withinTolerance(a, target uint8) bool {
	return int(a) >= int(target)-tolerance &&
		int(a) <= int(target)+tolerance
}

func extractRGB(color uint32) (uint8, uint8, uint8) {
	return uint8(color), // R
		uint8(color >> 8), // G
		uint8(color >> 16) // B
}

func clickLeftMouse() {
	var inputs [2]INPUT

	inputs[0].Type = INPUT_MOUSE
	inputs[0].Mi.DwFlags = MOUSEEVENTF_LEFTDOWN

	inputs[1].Type = INPUT_MOUSE
	inputs[1].Mi.DwFlags = MOUSEEVENTF_LEFTUP

	procSendInput.Call(
		2,
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(INPUT{}),
	)
}

// WinAPI обёртки
func getDC(hwnd uintptr) uintptr {
	ret, _, _ := procGetDC.Call(hwnd)
	return ret
}

func releaseDC(hwnd, hdc uintptr) {
	procReleaseDC.Call(hwnd, hdc)
}

func getPixel(hdc uintptr, x, y int) uint32 {
	ret, _, _ := procGetPixel.Call(hdc, uintptr(x), uintptr(y))
	return uint32(ret)
}

func getSystemMetrics(nIndex int) int {
	ret, _, _ := user32.NewProc("GetSystemMetrics").Call(uintptr(nIndex))
	return int(ret)
}
