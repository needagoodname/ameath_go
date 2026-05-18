package internal

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procSetTimer            = user32.NewProc("SetTimer")
	procKillTimer           = user32.NewProc("KillTimer")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procShowWindow          = user32.NewProc("ShowWindow")
	procGetWindowLong       = user32.NewProc("GetWindowLongW")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procWindowFromPoint     = user32.NewProc("WindowFromPoint")
)

// Windows API 常量
const (
	WS_EX_LAYERED       = 0x00080000
	WS_EX_TRANSPARENT   = 0x00000020
	WS_EX_TOOLWINDOW    = 0x00000080
	WS_EX_NOACTIVATE    = 0x08000000
	WS_POPUP            = 0x80000000
	WS_VISIBLE          = 0x10000000
	WS_CLIPSIBLINGS     = 0x04000000
	WM_LBUTTONDOWN      = 0x0201
	WM_LBUTTONUP        = 0x0202
	WM_MOUSEMOVE        = 0x0200
	WM_TIMER            = 0x0113
	WM_PAINT            = 0x000F
	WM_DESTROY          = 0x0002
	ULW_ALPHA           = 0x00000002
	AC_SRC_OVER         = 0x00
	AC_SRC_ALPHA        = 0x01
	SW_SHOWNOACTIVATE   = 4
	GWL_STYLE           = -16
)

type POINT struct{ X, Y int32 }
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}
type BLENDFUNCTION struct {
	BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte
}

// 创建窗口并运行消息循环
func RunMessageLoop() {
	className, _ := windows.UTF16PtrFromString("PetClass")
	
	wcex := &windows.WndClassEx{
		CbSize:        uint32(unsafe.Sizeof(windows.WndClassEx{})),
		LpfnWndProc:   windows.NewCallback(wndProc),
		HInstance:     windows.Handle(getModule()),
		HCursor:       windows.Handle(loadCursor(32512)),
		LpszClassName: className,
	}
	
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(wcex)))
	
	hwnd, _, _ := procCreateWindowEx.Call(
		WS_EX_LAYERED|WS_EX_TRANSPARENT|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)),
		0, WS_POPUP|WS_VISIBLE,
		uintptr(pet.X), uintptr(pet.Y),
		uintptr(pet.Width), uintptr(pet.Height),
		0, 0, uintptr(wcex.HInstance), 0,
	)
	
	pet.Hwnd = hwnd
	
	procSetTimer.Call(hwnd, 1, 16, 0)   // 60fps
	procSetTimer.Call(hwnd, 2, 2000, 0) // AI
	procSetTimer.Call(hwnd, 3, 5000, 0) // 遮挡检测
	
	var msg MSG
	for {
		select {
		case <-quitChan:
			return
		default:
			ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if ret == 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		render()
		return 0
		
	case WM_TIMER:
		if wParam == 1 {
			if !paused && !away {
				if pet.CurrentAnim != nil {
					pet.CurrentAnim.Update()
				}
				render()
			}
		} else if wParam == 2 {
			if !paused && !away {
				updateAI()
			}
		} else if wParam == 3 {
			if !paused && !pet.Dragging {
				away = checkOcclusion()
			}
		}
		return 0
		
	case WM_LBUTTONDOWN:
		pet.Dragging = true
		pet.DragX = int32(lParam) & 0xFFFF
		pet.DragY = int32(lParam) >> 16
		procSetCapture.Call(hwnd)
		switchAnim("click")
		return 0
		
	case WM_LBUTTONUP:
		pet.Dragging = false
		procReleaseCapture.Call(hwnd)
		Cfg.WindowX = pet.X
		Cfg.WindowY = pet.Y
		saveConfig()
		switchAnim("idle")
		return 0
		
	case WM_MOUSEMOVE:
		if pet.Dragging {
			x := int32(lParam) & 0xFFFF
			y := int32(lParam) >> 16
			pet.X += x - pet.DragX
			pet.Y += y - pet.DragY
			procSetWindowPos.Call(hwnd, 0, uintptr(pet.X), uintptr(pet.Y), 0, 0, 1|4)
		}
		return 0
		
	case WM_DESTROY:
		procKillTimer.Call(hwnd, 1)
		procKillTimer.Call(hwnd, 2)
		procKillTimer.Call(hwnd, 3)
		procPostQuitMessage.Call(0)
		return 0
	}
	
	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func getModule() uintptr {
	h, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	return h
}

func loadCursor(id uintptr) uintptr {
	c, _, _ := procLoadCursor.Call(0, id)
	return c
}

func checkOcclusion() bool {
	if pet.Hwnd == 0 {
		return false
	}
	pts := []POINT{
		{pet.X + pet.Width/2, pet.Y + pet.Height/2},
		{pet.X + 2, pet.Y + 2},
		{pet.X + pet.Width - 2, pet.Y + 2},
		{pet.X + 2, pet.Y + pet.Height - 2},
		{pet.X + pet.Width - 2, pet.Y + pet.Height - 2},
	}
	for _, pt := range pts {
		h, _, _ := procWindowFromPoint.Call(uintptr(uint32(pt.X)) | uintptr(uint32(pt.Y))<<32)
		if h == pet.Hwnd {
			return false
		}
	}
	return true
}