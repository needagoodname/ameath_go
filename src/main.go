package main

import (
	"math/rand"
	"runtime"
	"time"

	"golang.org/x/sys/windows"

	"github.com/na_me/ameath-go/internal"
)

var (
	pet      *Pet
	audioOn  = true
	quitChan = make(chan bool)
)

func init() {
	rand.Seed(time.Now().UnixNano())
	runtime.LockOSThread()
}

func main() {
	// 初始化宠物
	pet = &Pet{
		Name:   "爱弥斯",
		State:  "idle",
		X:      100, Y: 100,
		Width:  128, Height: 128,
		Anims:  make(map[string]*Animator),
		Sounds: make(map[string]string),
	}
	
	if err := internal.initAudio(&audioOn); err != nil {
		panic(err)
	}

	internal.loadResources(&pet)
	
	go systray.Run(internal.onTrayReady, internal.onTrayExit)
	createWindow()
}
