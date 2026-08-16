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

	cmdChan chan func()
}

// app 包级唯一实例。wndProc 回调签名固定无法带 receiver，经此访问。
var app *App

func NewApp() *App {
	app = &App{cmdChan: make(chan func(), 64)}
	return app
}

// postCmd 将状态变更投递到主线程执行（仅托盘 goroutine 调用）。
func (a *App) postCmd(f func()) {
	select {
	case a.cmdChan <- f:
		procPostMessage.Call(a.Pet.Hwnd, WM_APP_EXEC_CMD, 0, 0)
	default:
		println("command queue full, command dropped")
	}
}

// moveBy 相对移动窗口（仅主线程调用）。
func (a *App) moveBy(dx, dy int32) {
	p := a.Pet
	p.X += dx
	p.Y += dy
	procSetWindowPos.Call(p.Hwnd, 0, uintptr(p.X), uintptr(p.Y), 0, 0, 1|4)
}

// playHop happy 状态跳跃：3 次上下 20px，100ms 节奏。
// 自身是 goroutine，移动经 postCmd 在主线程执行。
func (a *App) playHop() {
	go func() {
		for i := 0; i < 3; i++ {
			a.postCmd(func() { a.moveBy(0, -20) })
			time.Sleep(100 * time.Millisecond)
			a.postCmd(func() { a.moveBy(0, 20) })
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
