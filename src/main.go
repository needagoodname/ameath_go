package main

import (
	"math/rand"
	"runtime"
	"time"

	"github.com/getlantern/systray"

	"github.com/na_me/ameath-go/internal"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	runtime.LockOSThread()
}

func main() {
	app := internal.NewApp()

	// 加载配置
	app.LoadConfig()

	// 发现可用宠物
	app.DiscoverPets()

	// 确定当前宠物
	petName := app.Cfg.CurrentPet
	avail := app.Pets
	if petName == "" && len(avail) > 0 {
		petName = avail[0]
	}
	if petName == "" {
		petName = "default"
	}
	app.NewPet(petName)

	// 初始化音频
	if err := app.InitAudio(); err != nil {
		println("Audio init failed:", err.Error())
	}

	// 加载宠物资源
	if err := app.LoadResources(); err != nil {
		println("Load resources failed:", err.Error())
	}

	// 同步开机自启状态
	app.SyncAutoStart()

	// 先建窗口再启托盘，保证 postCmd 投递时 hwnd 已存在
	app.CreateWindow()

	// 启动系统托盘和窗口消息循环
	go systray.Run(internal.OnTrayReady, internal.OnTrayExit)
	app.RunMessageLoop()

	// 消息循环退出后清理托盘
	systray.Quit()
}
