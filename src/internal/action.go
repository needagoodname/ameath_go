package internal

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unsafe"
)

// internalStates 是 AI 内部使用的状态，不暴露到托盘菜单
var internalStates = map[string]bool{
	"walk":  true,
	"click": true,
}

// assetsRoot 返回资源根目录（exe 同目录下的 assets，失败回退 ./assets）
func assetsRoot() string {
	exe, err := exePath()
	if err != nil {
		return filepath.Join(".", "assets")
	}
	return filepath.Join(filepath.Dir(exe), "assets")
}

// MenuStates 返回所有可用宠物的菜单可见状态合集
func (a *App) MenuStates() []string {
	set := map[string]bool{}
	for _, name := range a.Pets {
		dir := filepath.Join(assetsRoot(), name)
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
func (a *App) HasMenuState(state string) bool {
	_, ok := a.Pet.Anims[state]
	return ok && !internalStates[state]
}

// switchAnim 切换动画（仅主线程调用）
func (a *App) switchAnim(state string) {
	p := a.Pet
	if a.Paused {
		return
	}
	if anim, ok := p.Anims[state]; ok && anim != p.CurrentAnim {
		if p.CurrentAnim != nil {
			p.CurrentAnim.Playing = false
		}
		p.CurrentAnim = anim
		p.CurrentAnim.Current = 0
		p.CurrentAnim.CurrentLoop = 0
		p.CurrentAnim.Playing = true
		p.CurrentAnim.LastUpdate = time.Now()
		p.State = state
		p.StateTimer = 0

		if a.AudioOn {
			a.playSound(state)
		}
	}
}

// updateAI 状态转移（2s tick，仅主线程）
func (a *App) updateAI() {
	p := a.Pet
	if p.Dragging {
		return
	}

	p.StateTimer++

	switch p.State {
	case "idle":
		if rand.Intn(10) == 0 {
			a.switchAnim("walk")
			p.TargetX, p.TargetY = a.clampToScreen(
				p.X+int32(rand.Intn(200)-100),
				p.Y+int32(rand.Intn(200)-100),
			)
		} else if p.StateTimer > 20 && rand.Intn(5) == 0 {
			a.switchAnim("sleep")
		}

	case "walk":
		// 移动由 updateMovement（60fps tick）执行，此处仅兜底检查
		if p.X == p.TargetX && p.Y == p.TargetY {
			a.switchAnim("idle")
		}

	case "sleep":
		if p.StateTimer > 30 || rand.Intn(10) == 0 {
			a.switchAnim("idle")
			p.StateTimer = 0
		}

	case "click", "eat", "happy":
		if p.StateTimer > 5 {
			a.switchAnim("idle")
			p.StateTimer = 0
		}
	}
}

// updateMovement 向目标移动（60fps tick，仅主线程）
func (a *App) updateMovement() {
	p := a.Pet
	if p.State != "walk" || p.Dragging {
		return
	}
	dx := p.TargetX - p.X
	dy := p.TargetY - p.Y
	dist := math.Sqrt(float64(dx*dx + dy*dy))
	if dist < 1 {
		a.switchAnim("idle")
		return
	}

	speed := 2.0 // px/帧 ≈ 120px/s
	if dist <= speed {
		p.X = p.TargetX
		p.Y = p.TargetY
	} else {
		p.X += int32(float64(dx) * speed / dist)
		p.Y += int32(float64(dy) * speed / dist)
	}
	procSetWindowPos.Call(p.Hwnd, 0, uintptr(p.X), uintptr(p.Y), 0, 0, 1|4)

	if p.X == p.TargetX && p.Y == p.TargetY {
		a.switchAnim("idle")
	}
}

// DiscoverPets 扫描 assets 目录发现可用宠物
func (a *App) DiscoverPets() {
	entries, err := os.ReadDir(assetsRoot())
	if err != nil {
		return
	}
	a.Pets = nil
	for _, e := range entries {
		if e.IsDir() {
			a.Pets = append(a.Pets, e.Name())
		}
	}
}

// LoadResources 加载当前宠物的 GIF 动画和音频文件
// 资产结构: assets/{petName}/{state}/{name}.gif + {name}.mp3/wav
// 资源先加载到临时 map，全部成功后才提交（供 SwitchPet 回滚）。
func (a *App) LoadResources() error {
	p := a.Pet
	petDir := filepath.Join(assetsRoot(), p.Name)
	entries, err := os.ReadDir(petDir)
	if err != nil {
		return err
	}

	anims := make(map[string]*Animator)
	feetOf := make(map[string]int)
	sounds := make(map[string][]string)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		state := e.Name()
		stateDir := filepath.Join(petDir, state)

		// 加载目录下全部 GIF 并合并为一个动画
		gifs, _ := filepath.Glob(filepath.Join(stateDir, "*.gif"))
		if len(gifs) > 0 {
			if anim, feet, err := loadGIFs(gifs); err == nil {
				anims[state] = anim
				feetOf[state] = feet
			} else {
				println("Failed to load gifs in", stateDir, ":", err.Error())
			}
		}

		// 加载所有音频文件（.mp3 和 .wav）
		var files []string
		for _, ext := range []string{"*.mp3", "*.wav"} {
			f, _ := filepath.Glob(filepath.Join(stateDir, ext))
			files = append(files, f...)
		}
		if len(files) > 0 {
			sounds[state] = files
		}
	}

	// 全局归一化：统一画布尺寸与脚线，避免状态切换时精灵跳动
	baseW, baseH, baseFeet := 0, 0, 0
	for _, anim := range anims {
		if anim.Width > baseW {
			baseW = anim.Width
		}
		if anim.Height > baseH {
			baseH = anim.Height
		}
	}
	for _, feet := range feetOf {
		if feet > baseFeet {
			baseFeet = feet
		}
	}
	for state, anim := range anims {
		anim.normalizeFrames(baseW, baseH, baseFeet-feetOf[state])
	}

	// 默认动画：优先 idle，否则取第一个；无任何动画视为加载失败
	var base *Animator
	if anim, ok := anims["idle"]; ok {
		base = anim
	} else {
		for _, anim := range anims {
			base = anim
			break
		}
	}
	if base == nil {
		return fmt.Errorf("no gif found in %s", petDir)
	}

	p.Anims = anims
	p.Sounds = sounds
	p.BaseWidth = int32(base.Width)
	p.BaseHeight = int32(base.Height)
	p.SetScale(a.Cfg.ScalePercent)
	p.CurrentAnim = base
	p.CurrentAnim.Playing = true
	p.CurrentAnim.LastUpdate = time.Now()
	println("loaded", p.Name, "anims:", len(anims), "sounds:", len(sounds),
		"base:", p.BaseWidth, "x", p.BaseHeight, "scale:", a.Cfg.ScalePercent)
	return nil
}

// SwitchPet 切换到另一个宠物（失败时回滚，原状态不受影响）
func (a *App) SwitchPet(name string) error {
	p := a.Pet
	petDir := filepath.Join(assetsRoot(), name)
	info, err := os.Stat(petDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("pet %q not found", name)
	}

	oldAnims := p.Anims
	oldSounds := p.Sounds
	oldCurrent := p.CurrentAnim
	oldName := p.Name

	p.Anims = make(map[string]*Animator)
	p.Sounds = make(map[string][]string)
	p.CurrentAnim = nil
	p.Name = name

	if err := a.LoadResources(); err != nil {
		p.Anims = oldAnims
		p.Sounds = oldSounds
		p.CurrentAnim = oldCurrent
		p.Name = oldName
		return err
	}
	p.State = "idle"
	p.StateTimer = 0

	a.Cfg.CurrentPet = name
	a.saveConfig()

	// 更新窗口大小以匹配新宠物和当前缩放
	procSetWindowPos.Call(p.Hwnd, 0, 0, 0,
		uintptr(p.Width), uintptr(p.Height),
		0x0002|0x0004)

	return nil
}

// renderDbg 记录各早退点是否已打印过（仅主线程访问）
var renderDbg = struct{ guard, frame, dc, dib bool }{}

// render 渲染当前帧到分层窗口（仅主线程调用）
func (a *App) render() {
	p := a.Pet
	if p.Hwnd == 0 || p.CurrentAnim == nil {
		if !renderDbg.guard {
			renderDbg.guard = true
			println("render skip: hwnd:", p.Hwnd, "anim:", p.CurrentAnim)
		}
		return
	}

	frame := p.CurrentAnim.GetFrame()
	if frame == nil {
		if !renderDbg.frame {
			renderDbg.frame = true
			println("render skip: frame nil, current:", p.CurrentAnim.Current, "len:", len(p.CurrentAnim.Frames))
		}
		return
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 && !renderDbg.dc {
		renderDbg.dc = true
		println("GetDC failed, err:", getLastError())
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	defer procDeleteDC.Call(memDC)

	// DIB 头（布局必须与 Win32 BITMAPINFOHEADER 完全一致，共 40 字节）
	type BITMAPINFOHEADER struct {
		Size          uint32
		Width         int32
		Height        int32
		Planes        uint16
		BitCount      uint16
		Compression   uint32
		SizeImage     uint32
		XPelsPerMeter int32
		YPelsPerMeter int32
		ClrUsed       uint32
		ClrImportant  uint32
	}

	bmi := struct {
		Header BITMAPINFOHEADER
	}{
		Header: BITMAPINFOHEADER{
			Size:     uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			Width:    int32(p.Width),
			Height:   -p.Height, // 自顶向下
			Planes:   1,
			BitCount: 32,
		},
	}

	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(screenDC, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 {
		if !renderDbg.dib {
			renderDbg.dib = true
			println("CreateDIBSection failed, err:", getLastError(), "screenDC:", screenDC, "memDC:", memDC)
		}
		return
	}
	defer procDeleteObject.Call(hbm)

	procSelectObject.Call(memDC, hbm)

	// 复制像素 BGRA（最近邻缩放）
	pixels := (*[1 << 20]byte)(bits)
	srcW := int(p.BaseWidth)
	srcH := int(p.BaseHeight)
	dstW := int(p.Width)
	dstH := int(p.Height)
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
	dst := POINT{p.X, p.Y}
	size := POINT{p.Width, p.Height}

	ret, _, _ := procUpdateLayeredWindow.Call(
		p.Hwnd, screenDC,
		uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		memDC, uintptr(unsafe.Pointer(&src)),
		0, uintptr(unsafe.Pointer(&blend)), ULW_ALPHA,
	)
	if ret == 0 {
		println("UpdateLayeredWindow failed")
	} else if !a.RenderedOnce {
		a.RenderedOnce = true
		println("first render ok at", p.X, ",", p.Y, "size:", p.Width, "x", p.Height)
	}
}

func (a *App) resizeWindow(percent int) {
	a.Pet.SetScale(percent)
	procSetWindowPos.Call(a.Pet.Hwnd, 0, 0, 0,
		uintptr(a.Pet.Width), uintptr(a.Pet.Height),
		0x0002|0x0004) // SWP_NOMOVE | SWP_NOZORDER
	a.Cfg.ScalePercent = percent
	a.saveConfig()
}
