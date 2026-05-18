package internal

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unsafe"
)

var (
	availablePets []string
)

// internalStates 是 AI 内部使用的状态，不暴露到托盘菜单
var internalStates = map[string]bool{
	"walk":  true,
	"click": true,
}

// MenuStates 返回所有可用宠物的菜单可见状态合集
func MenuStates() []string {
	set := map[string]bool{}
	for _, name := range availablePets {
		dir := filepath.Join(".", "assets", name)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() && !internalStates[e.Name()] {
				set[e.Name()] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// HasMenuState 判断当前宠物是否有某个菜单可见状态
func HasMenuState(state string) bool {
	_, ok := pet.Anims[state]
	return ok && !internalStates[state]
}

// 切换动画
func switchAnim(state string) {
	if paused {
		return
	}
	if anim, ok := pet.Anims[state]; ok && anim != pet.CurrentAnim {
		if pet.CurrentAnim != nil {
			pet.CurrentAnim.Playing = false
		}
		pet.CurrentAnim = anim
		pet.CurrentAnim.Current = 0
		pet.CurrentAnim.CurrentLoop = 0
		pet.CurrentAnim.Playing = true
		pet.CurrentAnim.LastUpdate = time.Now()
		pet.State = state
		pet.StateTimer = 0
		
		if audioOn {
			playSound(state)
		}
	}
}

// AI 行为
func updateAI() {
	if pet.Dragging {
		return
	}
	
	pet.StateTimer++
	
	switch pet.State {
	case "idle":
		if rand.Intn(10) == 0 {
			switchAnim("walk")
			pet.TargetX = pet.X + int32(rand.Intn(200)-100)
			pet.TargetY = pet.Y + int32(rand.Intn(200)-100)
			
			// 边界限制
			if pet.TargetX < 0 {
				pet.TargetX = 0
			}
			if pet.TargetX > 1920-pet.Width {
				pet.TargetX = 1920 - pet.Width
			}
			if pet.TargetY < 0 {
				pet.TargetY = 0
			}
			if pet.TargetY > 1080-pet.Height {
				pet.TargetY = 1080 - pet.Height
			}
		} else if pet.StateTimer > 20 && rand.Intn(5) == 0 {
			switchAnim("sleep")
		}
		
	case "walk":
		dx := pet.TargetX - pet.X
		dy := pet.TargetY - pet.Y
		dist := sqrt(dx*dx + dy*dy)
		
		if dist < 5 {
			switchAnim("idle")
		} else {
			speed := int32(3)
			pet.X += dx * speed / dist
			pet.Y += dy * speed / dist
			procSetWindowPos.Call(pet.Hwnd, 0, uintptr(pet.X), uintptr(pet.Y), 0, 0, 1|4)
		}
		
	case "sleep":
		if pet.StateTimer > 30 || rand.Intn(10) == 0 {
			switchAnim("idle")
			pet.StateTimer = 0
		}
		
	case "click", "eat", "happy":
		if pet.StateTimer > 5 {
			switchAnim("idle")
			pet.StateTimer = 0
		}
	}
}

// DiscoverPets 扫描 assets 目录发现可用宠物
func DiscoverPets() {
	entries, err := os.ReadDir(filepath.Join(".", "assets"))
	if err != nil {
		return
	}
	availablePets = nil
	for _, e := range entries {
		if e.IsDir() {
			availablePets = append(availablePets, e.Name())
		}
	}
}

// AvailablePets 返回已发现的宠物列表
func AvailablePets() []string {
	return availablePets
}

// LoadResources 加载指定宠物的 GIF 动画和音频文件
// 资产结构: ./assets/{petName}/{state}/{name}.gif + {name}.mp3/wav
func LoadResources(petName string) {
	petDir := filepath.Join(".", "assets", petName)
	entries, err := os.ReadDir(petDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		state := e.Name()
		stateDir := filepath.Join(petDir, state)

		// 加载 GIF（取目录下第一个 .gif）
		gifs, _ := filepath.Glob(filepath.Join(stateDir, "*.gif"))
		if len(gifs) > 0 {
			if anim, err := loadGIF(gifs[0]); err == nil {
				pet.Anims[state] = anim
			} else {
				println("Failed to load", gifs[0], ":", err.Error())
			}
		}

		// 加载所有音频文件（.mp3 和 .wav）
		var sounds []string
		for _, ext := range []string{"*.mp3", "*.wav"} {
			files, _ := filepath.Glob(filepath.Join(stateDir, ext))
			sounds = append(sounds, files...)
		}
		if len(sounds) > 0 {
			pet.Sounds[state] = sounds
		}
	}

	// 默认动画：优先 idle，否则取第一个
	if anim, ok := pet.Anims["idle"]; ok {
		pet.CurrentAnim = anim
	} else {
		for _, anim := range pet.Anims {
			pet.CurrentAnim = anim
			break
		}
	}
	if pet.CurrentAnim != nil {
		pet.CurrentAnim.Playing = true
		pet.CurrentAnim.LastUpdate = time.Now()
	}
}

// SwitchPet 切换到另一个宠物
func SwitchPet(petName string) error {
	petDir := filepath.Join(".", "assets", petName)
	info, err := os.Stat(petDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("pet %q not found", petName)
	}

	pet.Anims = make(map[string]*Animator)
	pet.Sounds = make(map[string][]string)
	pet.CurrentAnim = nil
	pet.Name = petName
	LoadResources(petName)

	Cfg.CurrentPet = petName
	saveConfig()

	// 更新窗口大小以匹配新宠物和当前缩放
	procSetWindowPos.Call(pet.Hwnd, 0, 0, 0,
		uintptr(pet.Width), uintptr(pet.Height),
		0x0002|0x0004)

	return nil
}

// 渲染
func render() {
	if pet.Hwnd == 0 || pet.CurrentAnim == nil {
		return
	}
	
	frame := pet.CurrentAnim.GetFrame()
	if frame == nil {
		return
	}
	
	screenDC, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screenDC)
	
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	defer procDeleteDC.Call(memDC)
	
	// DIB 头
	type BITMAPINFOHEADER struct {
		Size, Width, Height, Planes, BitCount, Compression, SizeImage, XPels, YPels, ClrUsed, ClrImportant uint32
	}
	
	bmi := struct {
		Header BITMAPINFOHEADER
	}{
		Header: BITMAPINFOHEADER{
			Size:     uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			Width:    uint32(pet.Width),
			Height:   uint32(-pet.Height), // 自顶向下
			Planes:   1,
			BitCount: 32,
		},
	}
	
	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(screenDC, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 {
		return
	}
	defer procDeleteObject.Call(hbm)
	
	procSelectObject.Call(memDC, hbm)
	
	// 复制像素 BGRA（最近邻缩放）
	pixels := (*[1 << 20]byte)(bits)
	srcW := int(pet.BaseWidth)
	srcH := int(pet.BaseHeight)
	dstW := int(pet.Width)
	dstH := int(pet.Height)
	for dy := 0; dy < dstH; dy++ {
		sy := dy * srcH / dstH
		for dx := 0; dx < dstW; dx++ {
			sx := dx * srcW / dstW
			c := frame.RGBAAt(sx, sy)
			idx := (dy*dstW + dx) * 4
			pixels[idx] = c.B
			pixels[idx+1] = c.G
			pixels[idx+2] = c.R
			pixels[idx+3] = c.A
		}
	}
	
	blend := BLENDFUNCTION{AC_SRC_OVER, 0, 255, AC_SRC_ALPHA}
	src := POINT{0, 0}
	dst := POINT{pet.X, pet.Y}
	size := POINT{pet.Width, pet.Height}
	
	procUpdateLayeredWindow.Call(
		pet.Hwnd, screenDC,
		uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		memDC, uintptr(unsafe.Pointer(&src)),
		0, uintptr(unsafe.Pointer(&blend)), ULW_ALPHA,
	)
}

func sqrt(x int32) int32 {
	if x <= 0 {
		return 0
	}
	z := int32(1)
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func resizeWindow(percent int) {
	pet.SetScale(percent)
	procSetWindowPos.Call(pet.Hwnd, 0, 0, 0,
		uintptr(pet.Width), uintptr(pet.Height),
		0x0002|0x0004) // SWP_NOMOVE | SWP_NOZORDER
	Cfg.ScalePercent = percent
	saveConfig()
}