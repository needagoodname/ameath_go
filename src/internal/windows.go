package internal

import (
	"fmt"
	"time"
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
	procSetWindowLong       = user32.NewProc("SetWindowLongW")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procLoadCursor          = user32.NewProc("LoadCursorW")
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
	WS_EX_TOPMOST       = 0x00000008
	WS_POPUP            = 0x80000000
	WS_VISIBLE          = 0x10000000
	WS_CLIPSIBLINGS     = 0x04000000
	WM_LBUTTONDOWN      = 0x0201
	WM_LBUTTONUP        = 0x0202
	WM_RBUTTONUP        = 0x0205
	WM_MOUSEMOVE        = 0x0200
	WM_CAPTURECHANGED   = 0x0215
	WM_TIMER            = 0x0113
	WM_PAINT            = 0x000F
	WM_DESTROY          = 0x0002
	ULW_ALPHA           = 0x00000002
	AC_SRC_OVER         = 0x00
	AC_SRC_ALPHA        = 0x01
	SW_SHOWNOACTIVATE   = 4
	GWL_STYLE           = -16
	GWL_EXSTYLE         = -20

	// SetWindowPos 的 hWndInsertAfter 取值
	HWND_TOP       = 0
	HWND_BOTTOM    = 1
	HWND_TOPMOST   = ^uintptr(0) // (HWND)-1
	HWND_NOTOPMOST = ^uintptr(1) // (HWND)-2

	// SetWindowPos 标志位
	SWP_NOSIZE     = 0x0001
	SWP_NOMOVE     = 0x0002
	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010

	WM_APP              = 0x8000
	WM_APP_EXEC_CMD     = WM_APP + 1
	WM_APP_QUIT         = WM_APP + 2
	WM_DISPLAYCHANGE    = 0x021E
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
	// WS_EX_TOPMOST：桌宠置顶，避免被普通窗口盖住（盖住后旧的遮挡检测
	// 会自锁导致画面冻结）。先不带 WS_VISIBLE，等首帧渲染成功后再显示，
	// 避免启动时闪现黑框。置顶与否可在菜单切换并持久化（Cfg.AlwaysOnTop）。
	exStyle := uintptr(WS_EX_LAYERED) | uintptr(WS_EX_TOOLWINDOW) | uintptr(WS_EX_NOACTIVATE)
	if a.Cfg.AlwaysOnTop {
		exStyle |= WS_EX_TOPMOST
	}
	hwnd, _, _ := procCreateWindowEx.Call(
		exStyle,
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
	println("timers:", t1, t2)
}

// isTopmost 判断当前窗口扩展样式是否置顶（仅主线程调用）。
func (a *App) isTopmost() bool {
	if a.Pet == nil || a.Pet.Hwnd == 0 {
		return a.Cfg.AlwaysOnTop
	}
	gwl := int32(GWL_EXSTYLE)
	ex, _, _ := procGetWindowLong.Call(a.Pet.Hwnd, uintptr(gwl))
	return ex&WS_EX_TOPMOST != 0
}

// setTopmost 设置/取消窗口置顶（仅主线程调用）。
// 置顶：设置 WS_EX_TOPMOST 后以 HWND_TOPMOST 重新插入 Z 序顶部，
// 让宠物压过普通窗口及其它置顶窗口；取消：清除样式并降回 HWND_NOTOPMOST。
func (a *App) setTopmost(on bool) {
	if a.Pet == nil || a.Pet.Hwnd == 0 {
		return
	}
	gwl := int32(GWL_EXSTYLE)
	ex, _, _ := procGetWindowLong.Call(a.Pet.Hwnd, uintptr(gwl))
	if on {
		procSetWindowLong.Call(a.Pet.Hwnd, uintptr(gwl), ex|WS_EX_TOPMOST)
		procSetWindowPos.Call(a.Pet.Hwnd, HWND_TOPMOST, 0, 0, 0, 0,
			SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
	} else {
		procSetWindowLong.Call(a.Pet.Hwnd, uintptr(gwl), ex&^WS_EX_TOPMOST)
		procSetWindowPos.Call(a.Pet.Hwnd, HWND_NOTOPMOST, 0, 0, 0, 0,
			SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
	}
	a.Cfg.AlwaysOnTop = on
}

// lastTopmostAssert 周期置顶节流（仅主线程访问）。
var lastTopmostAssert time.Time

// assertTopmost 置顶开启时，周期性把宠物重新顶到 Z 序最上方（仅主线程调用）。
// 浏览器/播放器等窗口进入全屏或获焦时可能把自己抬到置顶层盖住桌宠；
// 这里每 ~250ms 用 HWND_TOPMOST 重插一次，保证桌宠始终压在最上面
// （包括浏览器全屏）。置顶关闭时不做任何事。
func (a *App) assertTopmost() {
	now := time.Now()
	if now.Sub(lastTopmostAssert) < 250*time.Millisecond {
		return
	}
	lastTopmostAssert = now
	if a.Pet == nil || a.Pet.Hwnd == 0 {
		return
	}
	if !a.isTopmost() {
		return
	}
	// 右键菜单弹出期间暂停重顶：否则桌宠会把自己顶到菜单上方把菜单盖住。
	if a.MenuOpen {
		return
	}
	procSetWindowPos.Call(a.Pet.Hwnd, HWND_TOPMOST, 0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
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
			// 诊断：每秒打印一次状态（控制台版本可见）
			app.debugTick()
			// 拖拽看门狗：超过 10s 无鼠标输入仍处于拖拽则强制释放，
			// 兜底 WM_LBUTTONUP / WM_CAPTURECHANGED 都未收到的情况，
			// 避免 Dragging 卡死导致所有动作无法结束/切回 idle。
			if app.Pet.Dragging && time.Since(app.Pet.DragActivity) > 10*time.Second {
				app.releaseDrag()
			}
			// 置顶开启时周期性重插 Z 序顶部，防止浏览器获焦/全屏后盖住桌宠。
			app.assertTopmost()
			if !app.Paused {
				if app.Pet.CurrentAnim != nil {
					app.Pet.CurrentAnim.Update()
				}
				app.maybeFinishBehavior()
				app.updateMovement()
				// 始终渲染：原“被遮挡时跳过渲染”依赖 WindowFromPoint 对分层窗口
				// 的命中测试，表面一旦陈旧就自锁（away 恒 true → 永不渲染，
				// 画面停在最后一帧）。宠物已置顶（WS_EX_TOPMOST），始终渲染即可。
				app.render()
			}
		} else if wParam == 2 {
			if !app.Paused {
				app.updateAI()
			}
		}
		return 0

	case WM_LBUTTONDOWN:
		app.Pet.Dragging = true
		app.Pet.DragMoved = false
		app.Pet.DragActivity = time.Now()
		app.Pet.DragX = int32(int16(lParam & 0xFFFF))
		app.Pet.DragY = int32(int16(lParam >> 16))
		procSetCapture.Call(hwnd)
		wasClick := app.Pet.State == "click"
		app.switchAnim("click")
		// 快速连点（仍处于 click 状态）时 switchAnim 因动画相同不会重播音效，
		// 这里补播点击音效，保证每次点击都响应“嗯”（voice.txt：点击：嗯）。
		if wasClick {
			app.playSound("click")
		}
		return 0

	case WM_LBUTTONUP:
		moved := app.Pet.DragMoved
		app.releaseDrag()
		app.Cfg.WindowX = app.Pet.X
		app.Cfg.WindowY = app.Pet.Y
		app.saveConfig()
		// 快速点击（未拖动）保持 click 状态，让反应动画可见；
		// 拖动结束则立即回到 idle2。
		if moved {
			app.switchToIdle2()
		}
		return 0

	case WM_CAPTURECHANGED:
		// 鼠标捕获被系统或其他窗口夺走（通常意味着在窗口外释放 / Alt-Tab 等），
		// 立即结束拖拽，避免 Dragging 永久卡死导致动作无法切回 idle。
		app.releaseDrag()
		return 0

	case WM_MOUSEMOVE:
		if app.Pet.Dragging {
			app.Pet.DragActivity = time.Now()
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

	case WM_DISPLAYCHANGE:
		// 显示器分辨率 / 工作区变化（插拔显示器、RDP、DPI 调整）后重采工作区
		// 并把宠物拉回可见范围，避免越界或被任务栏遮挡。
		var work RECT
		procSystemParameters.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&work)), 0)
		if work.Right-work.Left > 0 && work.Bottom-work.Top > 0 {
			app.ScreenX = work.Left
			app.ScreenY = work.Top
			app.ScreenW = work.Right - work.Left
			app.ScreenH = work.Bottom - work.Top
		}
		app.Pet.X, app.Pet.Y = app.clampToScreen(app.Pet.X, app.Pet.Y)
		procSetWindowPos.Call(app.Pet.Hwnd, 0, uintptr(app.Pet.X), uintptr(app.Pet.Y), 0, 0, 1|4)
		return 0

	case WM_DESTROY:
		// 标记退出，使在途 postCmd 立即返回，避免向已销毁窗口投递
		app.quit = true
		app.hopToken++
		procKillTimer.Call(hwnd, 1)
		procKillTimer.Call(hwnd, 2)
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
	appendMenuString(menu, MF_STRING, 2, "暂停")
	topTitle := "置顶"
	if a.isTopmost() {
		topTitle = "✓ 置顶"
	}
	appendMenuString(menu, MF_STRING, 3, topTitle)
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

	volumePresets := []int{0, 25, 50, 75, 100}
	volumeMenu, _, _ := procCreatePopupMenu.Call()
	for i, pct := range volumePresets {
		title := fmt.Sprintf("%d%%", pct)
		if pct == a.Cfg.VolumePercent {
			title = "✓ " + title
		}
		appendMenuString(volumeMenu, MF_STRING, uintptr(600+i), title)
	}
	appendMenuString(menu, MF_POPUP, volumeMenu, "音量")

	appendMenuString(menu, MF_SEPARATOR, 0, "")
	appendMenuString(menu, MF_STRING, 5, "开机启动")
	appendMenuString(menu, MF_SEPARATOR, 0, "")
	appendMenuString(menu, MF_STRING, 6, "退出")

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// 菜单弹出期间置 MenuOpen，暂停周期置顶，避免桌宠盖住自己的右键菜单；
	// TrackPopupMenu 在菜单关闭前阻塞，期间 WM_TIMER 仍会派发。
	a.MenuOpen = true
	cmd, _, _ := procTrackPopupMenu.Call(
		menu, TPM_RETURNCMD|TPM_NONOTIFY|TPM_RIGHTBUTTON,
		uintptr(pt.X), uintptr(pt.Y), 0, a.Pet.Hwnd, 0,
	)
	a.MenuOpen = false
	// 菜单关闭后立即恢复置顶，避免桌宠停留被盖的位置
	a.assertTopmost()

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
		a.Paused = !a.Paused
	case cmd == 3:
		a.setTopmost(!a.isTopmost())
		a.saveConfig()
	case cmd == 5:
		a.Cfg.AutoStart = !a.Cfg.AutoStart
		if a.Cfg.AutoStart {
			EnableAutoStart()
		} else {
			DisableAutoStart()
		}
		a.saveConfig()
	case cmd == 6:
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
	case cmd >= 600 && cmd < 700:
		if i := int(cmd - 600); i < len(volumePresets) {
			if pct := volumePresets[i]; pct != a.Cfg.VolumePercent {
				a.Cfg.VolumePercent = pct
				a.saveConfig()
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
