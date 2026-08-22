package internal

import (
	"time"
)

// App 聚合全部应用状态。
// 所有权规则：除 cmdChan 外，所有字段仅由主线程（消息循环/wndProc/定时器）
// 读写；托盘 goroutine 通过 postCmd 投递闭包到主线程执行。
type App struct {
	Pet     *Pet
	Cfg     Config
	AudioOn bool
	Paused  bool
	Away    bool
	Pets    []string
	ScreenW int32
	ScreenH int32
	ScreenX int32
	ScreenY int32

	RenderedOnce bool
	TrayIcon     []byte

	// hopToken 用于让在途 playHop goroutine 检测到自己已过期而自行终止。
	// 切换宠物 / 窗口销毁时递增。
	hopToken uint64

	// quit 在 WM_DESTROY 置位，之后 postCmd 不再入队，避免托盘 goroutine
	// 向已销毁窗口投递命令导致死锁或写到非法 hwnd。
	quit bool

	cmdChan chan func()
}

// app 包级唯一实例。wndProc 回调签名固定无法带 receiver，经此访问。
var app *App

func NewApp() *App {
	app = &App{cmdChan: make(chan func(), 256)}
	return app
}

// postCmd 将状态变更投递到主线程执行（仅托盘 goroutine 调用）。
// 采用阻塞 send：托盘 goroutine 阻塞不会影响主线程，主线程始终会排空 cmdChan；
// 用户命令（静音/暂停/缩放等）不会被静默丢失。quit 置位后立即返回，避免向已销毁窗口投递。
func (a *App) postCmd(f func()) {
	if a.quit {
		return
	}
	a.cmdChan <- f
	procPostMessage.Call(a.Pet.Hwnd, WM_APP_EXEC_CMD, 0, 0)
}

// moveBy 相对移动窗口（仅主线程调用）。
func (a *App) moveBy(dx, dy int32) {
	p := a.Pet
	p.X += dx
	p.Y += dy
	procSetWindowPos.Call(p.Hwnd, 0, uintptr(p.X), uintptr(p.Y), 0, 0, 1|4)
}

// releaseDrag 强制结束拖拽状态（正常释放 / 捕获丢失 WM_CAPTURECHANGED /
// 看门狗超时共用）。清除 Dragging 以解锁 AI、一次性行为结束与移动逻辑，
// 并复位 Away 恢复渲染，避免 WM_LBUTTONUP 丢失导致状态机永久冻结
// （表现为“所有动作结束后都无法切回 idle、画面停在最后一帧”）。
func (a *App) releaseDrag() {
	p := a.Pet
	if p == nil {
		return
	}
	p.Dragging = false
	p.DragMoved = false
	if p.Hwnd != 0 {
		procReleaseCapture.Call(p.Hwnd)
	}
	a.Away = false
}

// playHop happy 状态跳跃：3 次上下 20px，100ms 节奏。
// 自身是 goroutine，移动经 postCmd 在主线程执行。
// 捕获启动时的 pet 指针与 hopToken；若期间切换宠物或窗口销毁导致 token 变化，
// 后续闭包直接 return，不再作用于新宠物或已销毁窗口。
func (a *App) playHop() {
	token := a.hopToken
	pet := a.Pet
	go func() {
		for i := 0; i < 3; i++ {
			a.postCmd(func() {
				if a.hopToken != token || a.Pet != pet {
					return
				}
				a.moveBy(0, -20)
			})
			time.Sleep(100 * time.Millisecond)
			a.postCmd(func() {
				if a.hopToken != token || a.Pet != pet {
					return
				}
				a.moveBy(0, 20)
			})
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

// clampToScreen 将目标坐标限制在工作区（含任务栏避让）范围内。
func (a *App) clampToScreen(x, y int32) (int32, int32) {
	maxX := a.ScreenX + a.ScreenW - a.Pet.Width
	maxY := a.ScreenY + a.ScreenH - a.Pet.Height
	if x < a.ScreenX {
		x = a.ScreenX
	}
	if x > maxX {
		x = maxX
	}
	if y < a.ScreenY {
		y = a.ScreenY
	}
	if y > maxY {
		y = maxY
	}
	return x, y
}
