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
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSystemParameters    = user32.NewProc("SystemParametersInfoW")
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
	WM_RBUTTONUP        = 0x0205
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
	MF_STRING           = 0x0000
	MF_SEPARATOR        = 0x0800
	MF_POPUP            = 0x0010
	MF_GRAYED           = 0x0001
	TPM_RETURNCMD       = 0x0100
	TPM_NONOTIFY        = 0x0080
	TPM_RIGHTBUTTON     = 0x0002
	SPI_GETWORKAREA     = 0x0030
)

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

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

	// 取工作区（排除任务栏），避免宠物走到任务栏后面被判遮挡而卡住
	var work RECT
	procSystemParameters.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&work)), 0)
	if work.Right-work.Left <= 0 || work.Bottom-work.Top <= 0 {
		screenW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
		screenH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
		work.Left, work.Top = 0, 0
		work.Right = int32(screenW)
		work.Bottom = int32(screenH)
	}
	a.ScreenX = work.Left
	a.ScreenY = work.Top
	a.ScreenW = work.Right - work.Left
	a.ScreenH = work.Bottom - work.Top

	// 配置里保存的旧坐标可能已超出屏幕（换显示器/分辨率），拉回可见区域
	a.Pet.X, a.Pet.Y = a.clampToScreen(a.Pet.X, a.Pet.Y)

	frames := 0
	if a.Pet.CurrentAnim != nil {
		frames = len(a.Pet.CurrentAnim.Frames)
	}
	println("pet:", a.Pet.Name, "pos:", a.Pet.X, ",", a.Pet.Y,
		"size:", a.Pet.Width, "x", a.Pet.Height,
		"workarea:", a.ScreenX, ",", a.ScreenY, a.ScreenW, "x", a.ScreenH,
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
	// 先不带 WS_VISIBLE，等首帧渲染成功后再显示，避免启动时闪现黑框。
	hwnd, _, _ := procCreateWindowEx.Call(
		WS_EX_LAYERED|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)),
		0, WS_POPUP,
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
			if !app.Paused {
				if app.Pet.CurrentAnim != nil {
					app.Pet.CurrentAnim.Update()
				}
				app.updateMovement()
				// 被遮挡时仅跳过渲染（省 CPU），AI/移动照常，
				// 避免宠物卡在遮挡区域无法自行走出。
				if !app.Away {
					app.render()
				}
			}
		} else if wParam == 2 {
			if !app.Paused {
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
		app.Pet.DragMoved = false
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
		// 快速点击（未拖动）保持 click 状态，让反应动画可见；
		// 拖动结束则立即回到 idle。点击态由 AI 超时后回到 idle。
		if app.Pet.DragMoved {
			app.switchAnim("idle")
		}
		return 0

	case WM_MOUSEMOVE:
		if app.Pet.Dragging {
			x := int32(int16(lParam & 0xFFFF))
			y := int32(int16(lParam >> 16))
			app.Pet.X += x - app.Pet.DragX
			app.Pet.Y += y - app.Pet.DragY
			app.Pet.DragMoved = true
			procSetWindowPos.Call(hwnd, 0, uintptr(app.Pet.X), uintptr(app.Pet.Y), 0, 0, 1|4)
		}
		return 0

	case WM_RBUTTONUP:
		app.showContextMenu()
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

func appendMenuString(menu, flags, id uintptr, text string) {
	var p *uint16
	if text != "" {
		p, _ = windows.UTF16PtrFromString(text)
	}
	procAppendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(p)))
}

// showContextMenu 右键桌宠弹出菜单（主线程调用，与托盘菜单操作一致）。
func (a *App) showContextMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	// 行为状态（动态生成）
	states := a.MenuStates()
	for i, s := range states {
		appendMenuString(menu, MF_STRING, uintptr(400+i), s)
	}
	if len(states) > 0 {
		appendMenuString(menu, MF_SEPARATOR, 0, "")
	}

	appendMenuString(menu, MF_STRING, 1, "显示/隐藏")
	appendMenuString(menu, MF_STRING, 2, "静音")
	appendMenuString(menu, MF_STRING, 3, "暂停")
	appendMenuString(menu, MF_SEPARATOR, 0, "")

	petMenu, _, _ := procCreatePopupMenu.Call()
	for i, name := range a.Pets {
		title := name
		if name == a.Pet.Name {
			title = "✓ " + name
		}
		appendMenuString(petMenu, MF_STRING, uintptr(100+i), title)
	}
	appendMenuString(menu, MF_POPUP, petMenu, "切换宠物")

	scalePresets := []int{50, 75, 100, 125, 150, 200}
	scaleMenu, _, _ := procCreatePopupMenu.Call()
	for i, pct := range scalePresets {
		title := fmt.Sprintf("%d%%", pct)
		if pct == a.Cfg.ScalePercent {
			title = "✓ " + title
		}
		appendMenuString(scaleMenu, MF_STRING, uintptr(200+i), title)
	}
	appendMenuString(menu, MF_POPUP, scaleMenu, "缩放")

	appendMenuString(menu, MF_SEPARATOR, 0, "")
	appendMenuString(menu, MF_STRING, 6, "开机启动")
	appendMenuString(menu, MF_SEPARATOR, 0, "")
	appendMenuString(menu, MF_STRING, 7, "退出")

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	cmd, _, _ := procTrackPopupMenu.Call(
		menu, TPM_RETURNCMD|TPM_NONOTIFY|TPM_RIGHTBUTTON,
		uintptr(pt.X), uintptr(pt.Y), 0, a.Pet.Hwnd, 0,
	)

	switch {
	case cmd == 1:
		gwl := int32(GWL_STYLE)
		style, _, _ := procGetWindowLong.Call(a.Pet.Hwnd, uintptr(gwl))
		if style&WS_VISIBLE != 0 {
			procShowWindow.Call(a.Pet.Hwnd, 0)
		} else {
			procShowWindow.Call(a.Pet.Hwnd, SW_SHOWNOACTIVATE)
		}
	case cmd == 2:
		a.AudioOn = !a.AudioOn
		a.Cfg.AudioOn = a.AudioOn
		a.saveConfig()
	case cmd == 3:
		a.Paused = !a.Paused
		if !a.Paused {
			a.Away = false
		}
	case cmd == 6:
		a.Cfg.AutoStart = !a.Cfg.AutoStart
		if a.Cfg.AutoStart {
			EnableAutoStart()
		} else {
			DisableAutoStart()
		}
		a.saveConfig()
	case cmd == 7:
		procPostMessage.Call(a.Pet.Hwnd, WM_APP_QUIT, 0, 0)
	case cmd >= 100 && cmd < 200:
		if i := int(cmd - 100); i < len(a.Pets) {
			if err := a.SwitchPet(a.Pets[i]); err != nil {
				println("switch pet failed:", err.Error())
			}
		}
	case cmd >= 200 && cmd < 300:
		if i := int(cmd - 200); i < len(scalePresets) {
			if pct := scalePresets[i]; pct != a.Cfg.ScalePercent {
				a.resizeWindow(pct)
			}
		}
	case cmd >= 400:
		if i := int(cmd - 400); i < len(states) {
			s := states[i]
			a.switchAnim(s)
			if s == "happy" {
				a.playHop()
			}
		}
	}
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
