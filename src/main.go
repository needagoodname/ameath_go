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
	// 加载配置
	internal.LoadConfig()

	// 发现可用宠物
	internal.DiscoverPets()

	// 确定当前宠物
	petName := internal.Cfg.CurrentPet
	avail := internal.AvailablePets()
	if petName == "" && len(avail) > 0 {
		petName = avail[0]
	}
	if petName == "" {
		petName = "default"
	}

	// 创建宠物实例
	internal.NewPet(petName)

	// 初始化音频
	if err := internal.InitAudio(); err != nil {
		println("Audio init failed:", err.Error())
	}

	// 加载宠物资源
	internal.LoadResources(petName)

	// 同步开机自启状态
	internal.SyncAutoStart()

	// 启动系统托盘和窗口消息循环
	go systray.Run(internal.OnTrayReady, internal.OnTrayExit)
	internal.RunMessageLoop()
}
