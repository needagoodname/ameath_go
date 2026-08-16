package internal

import (
	"fmt"
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
	procPostMessage         = user32.NewProc("PostMessageW")
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
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	procGetLastError        = kernel32.NewProc("GetLastError")
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
	WM_APP              = 0x8000
	WM_APP_EXEC_CMD     = WM_APP + 1
	WM_APP_QUIT         = WM_APP + 2
	SM_CXSCREEN         = 0
	SM_CYSCREEN         = 1
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
type WndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

// CreateWindow 注册窗口类、创建分层窗口、启动定时器。
// 必须在托盘启动前调用，保证 postCmd 投递时 hwnd 已存在。
func (a *App) CreateWindow() {
	procSetProcessDPIAware.Call()

	screenW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	a.ScreenW = int32(screenW)
	a.ScreenH = int32(screenH)

	// 配置里保存的旧坐标可能已超出屏幕（换显示器/分辨率），拉回可见区域
	a.Pet.X, a.Pet.Y = a.clampToScreen(a.Pet.X, a.Pet.Y)

	frames := 0
	if a.Pet.CurrentAnim != nil {
		frames = len(a.Pet.CurrentAnim.Frames)
	}
	println("pet:", a.Pet.Name, "pos:", a.Pet.X, ",", a.Pet.Y,
		"size:", a.Pet.Width, "x", a.Pet.Height,
		"screen:", a.ScreenW, "x", a.ScreenH,
		"anims:", len(a.Pet.Anims), "frames:", frames)

	className, _ := windows.UTF16PtrFromString("PetClass")

	wcex := &WndClassEx{
		CbSize:        uint32(unsafe.Sizeof(WndClassEx{})),
		LpfnWndProc:   windows.NewCallback(wndProc),
		HInstance:     windows.Handle(getModule()),
		HCursor:       windows.Handle(loadCursor(32512)),
		LpszClassName: className,
	}

	r, _, _ := procRegisterClassEx.Call(uintptr(unsafe.Pointer(wcex)))
	if r == 0 {
		println("RegisterClassEx failed, err:", getLastError())
	}

	// 注意：不加 WS_EX_TRANSPARENT，否则整个窗口会变成鼠标穿透，
	// 宠物无法接收点击。透明区域点击穿透由 UpdateLayeredWindow 的
	// per-pixel alpha（ULW_ALPHA + AC_SRC_ALPHA）自动处理。
	hwnd, _, _ := procCreateWindowEx.Call(
		WS_EX_LAYERED|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)),
		0, WS_POPUP|WS_VISIBLE,
		uintptr(a.Pet.X), uintptr(a.Pet.Y),
		uintptr(a.Pet.Width), uintptr(a.Pet.Height),
		0, 0, uintptr(wcex.HInstance), 0,
	)

	a.Pet.Hwnd = hwnd
	if hwnd == 0 {
		println("CreateWindowEx failed, err:", getLastError())
		return
	}
	println("hwnd:", hwnd)

	t1, _, _ := procSetTimer.Call(hwnd, 1, 16, 0)   // 60fps：动画+移动+渲染
	t2, _, _ := procSetTimer.Call(hwnd, 2, 2000, 0) // AI 状态转移
	t3, _, _ := procSetTimer.Call(hwnd, 3, 5000, 0) // 遮挡检测
	println("timers:", t1, t2, t3)
}

// RunMessageLoop 消息泵。GetMessage 返回 0（WM_QUIT）时退出。
func (a *App) RunMessageLoop() {
	var msg MSG
	seen := map[uint32]bool{}
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			return
		}
		if ret == ^uintptr(0) {
			println("GetMessage failed, err:", getLastError())
			continue
		}
		if !seen[msg.Message] {
			seen[msg.Message] = true
			fmt.Printf("msg: 0x%x\n", msg.Message)
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

var wndProcCount int

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if wndProcCount < 20 {
		wndProcCount++
		fmt.Printf("wndproc#%d: msg=0x%x wParam=%d\n", wndProcCount, msg, wParam)
	}
	switch msg {
	case WM_PAINT:
		app.render()
		return 0

	case WM_TIMER:
		if wParam == 1 {
			if !app.Paused && !app.Away {
				if app.Pet.CurrentAnim != nil {
					app.Pet.CurrentAnim.Update()
				}
				app.updateMovement()
				app.render()
			}
		} else if wParam == 2 {
			if !app.Paused && !app.Away {
				app.updateAI()
			}
		} else if wParam == 3 {
			if !app.Paused && !app.Pet.Dragging {
				app.Away = app.checkOcclusion()
			}
		}
		return 0

	case WM_LBUTTONDOWN:
		app.Pet.Dragging = true
		app.Pet.DragX = int32(int16(lParam & 0xFFFF))
		app.Pet.DragY = int32(int16(lParam >> 16))
		procSetCapture.Call(hwnd)
		app.switchAnim("click")
		return 0

	case WM_LBUTTONUP:
		app.Pet.Dragging = false
		procReleaseCapture.Call(hwnd)
		app.Cfg.WindowX = app.Pet.X
		app.Cfg.WindowY = app.Pet.Y
		app.saveConfig()
		app.switchAnim("idle")
		return 0

	case WM_MOUSEMOVE:
		if app.Pet.Dragging {
			x := int32(int16(lParam & 0xFFFF))
			y := int32(int16(lParam >> 16))
			app.Pet.X += x - app.Pet.DragX
			app.Pet.Y += y - app.Pet.DragY
			procSetWindowPos.Call(hwnd, 0, uintptr(app.Pet.X), uintptr(app.Pet.Y), 0, 0, 1|4)
		}
		return 0

	// 托盘投递的命令在主线程执行
	case WM_APP_EXEC_CMD:
		for {
			select {
			case f := <-app.cmdChan:
				f()
			default:
				return 0
			}
		}

	case WM_APP_QUIT:
		procDestroyWindow.Call(hwnd) // 同步触发 WM_DESTROY
		return 0

	case WM_DESTROY:
		procKillTimer.Call(hwnd, 1)
		procKillTimer.Call(hwnd, 2)
		procKillTimer.Call(hwnd, 3)
		app.Cfg.WindowX = app.Pet.X
		app.Cfg.WindowY = app.Pet.Y
		app.saveConfig()
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func getLastError() uint32 {
	e, _, _ := procGetLastError.Call()
	return uint32(e)
}

func getModule() uintptr {
	h, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	return h
}

func loadCursor(id uintptr) uintptr {
	c, _, _ := procLoadCursor.Call(0, id)
	return c
}

// checkOcclusion 采样精灵不透明区域判断是否被其他窗口遮挡。
// 仅不透明像素（alpha ≥ 32）参与 WindowFromPoint 命中判断，
// 避免透明像素被误判为被遮挡。
func (a *App) checkOcclusion() bool {
	p := a.Pet
	if p.Hwnd == 0 || p.CurrentAnim == nil {
		return false
	}
	frame := p.CurrentAnim.GetFrame()
	if frame == nil {
		return false
	}

	pts := []POINT{
		{p.X + p.Width/2, p.Y + p.Height/2},
		{p.X + p.Width/4, p.Y + p.Height/2},
		{p.X + 3*p.Width/4, p.Y + p.Height/2},
		{p.X + p.Width/2, p.Y + p.Height/4},
		{p.X + p.Width/2, p.Y + 3*p.Height/4},
	}
	for _, pt := range pts {
		// 屏幕坐标换算为精灵坐标检查透明度
		sx := int(pt.X-p.X) * int(p.BaseWidth) / int(p.Width)
		sy := int(pt.Y-p.Y) * int(p.BaseHeight) / int(p.Height)
		if sx < 0 || sy < 0 || sx >= int(p.BaseWidth) || sy >= int(p.BaseHeight) {
			continue
		}
		if frame.RGBAAt(sx, sy).A < 32 {
			continue
		}
		h, _, _ := procWindowFromPoint.Call(uintptr(uint32(pt.X)) | uintptr(uint32(pt.Y))<<32)
		if h == p.Hwnd {
			return false
		}
	}
	return true
}
