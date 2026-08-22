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
	"idle":  true,
}

// oneShotStates 是一次性行为：动画播放完一轮后自动切回 idle2
var oneShotStates = map[string]bool{
	"click": true,
	"eat":   true,
	"happy": true,
}

// assetsRoot 返回资源根目录（exe 同目录下的 assets，失败回退 ./assets）
func assetsRoot() string {
	exe, err := exePath()
	if err != nil {
		return filepath.Join(".", "assets")
	}
	return filepath.Join(filepath.Dir(exe), "assets")
}

// MenuStates 返回所有可用宠物的菜单可见状态合集（只收录含 GIF 的状态目录）
func (a *App) MenuStates() []string {
	set := map[string]bool{}
	for _, name := range a.Pets {
		dir := filepath.Join(assetsRoot(), name)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() || internalStates[e.Name()] {
				continue
			}
			gifs, _ := filepath.Glob(filepath.Join(dir, e.Name(), "*.gif"))
			if len(gifs) > 0 {
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
	vs, ok := a.Pet.Anims[state]
	return ok && len(vs) > 0 && !internalStates[state]
}

// switchAnim 切换动画（仅主线程调用）。同状态多个变体（如 idle1~4）随机选取。
func (a *App) switchAnim(state string) {
	p := a.Pet
	if a.Paused {
		return
	}
	vs := p.Anims[state]
	if len(vs) == 0 {
		return
	}
	anim := vs[rand.Intn(len(vs))]
	if anim == p.CurrentAnim {
		return
	}
	a.setAnim(state, anim)
}

// setAnim 切换到指定动画（switchAnim/switchToIdle2 的公共逻辑）。
func (a *App) setAnim(state string, anim *Animator) {
	p := a.Pet
	if p.CurrentAnim != nil {
		p.CurrentAnim.Playing = false
	}
	changed := p.State != state
	p.CurrentAnim = anim
	p.CurrentAnim.Current = 0
	p.CurrentAnim.CurrentLoop = 0
	p.CurrentAnim.Playing = true
	p.CurrentAnim.LastUpdate = time.Now()
	p.State = state
	p.StateTimer = 0

	// 同状态换变体（如 idle 随机轮换）不重复播放音效
	if a.AudioOn && changed {
		a.playSound(state)
	}
}

// idle2Anim 返回 idle 第二变体（缺省第一个），无则返回 nil。
func (a *App) idle2Anim() *Animator {
	vs := a.Pet.Anims["idle"]
	if len(vs) == 0 {
		return nil
	}
	if len(vs) > 1 {
		return vs[1]
	}
	return vs[0]
}

// switchToIdle2 行为结束后切到 idle2（第二变体，缺省用第一个）。
func (a *App) switchToIdle2() {
	anim := a.idle2Anim()
	if anim == nil || anim == a.Pet.CurrentAnim {
		return
	}
	a.setAnim("idle", anim)
}

// maybeFinishBehavior 一次性行为动画播放完一轮后切回 idle2（每帧调用）。
func (a *App) maybeFinishBehavior() {
	p := a.Pet
	if p.CurrentAnim == nil || !oneShotStates[p.State] {
		return
	}
	if p.Dragging {
		return
	}
	// 播满一圈，或动画已因有限 LoopCount 自行停止（Playing=false）→ 都切回 idle2，
	// 不依赖单一条件，避免“播完停在某帧不切换”。
	if p.CurrentAnim.CurrentLoop >= 1 || !p.CurrentAnim.Playing {
		a.switchToIdle2()
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
			// 随机方向 + 最小距离，保证走动足够久以完整播放 move.gif
			angle := rand.Float64() * 2 * math.Pi
			dist := 150 + rand.Float64()*250
			p.TargetX, p.TargetY = a.clampToScreen(
				p.X+int32(math.Cos(angle)*dist),
				p.Y+int32(math.Sin(angle)*dist),
			)
		} else if p.StateTimer > 20 && rand.Intn(5) == 0 {
			a.switchAnim("sleep")
		}

	case "walk":
		// 移动由 updateMovement（60fps tick）执行，此处仅兜底检查
		if p.X == p.TargetX && p.Y == p.TargetY {
			a.switchToIdle2()
		}

	case "sleep":
		if p.StateTimer > 30 || rand.Intn(10) == 0 {
			a.switchToIdle2()
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
	// 移动期间强制循环 walk 动画：即便 GIF 自带有限 LoopCount（播完
	// Playing 变 false），此处重置使其在移动结束前持续播放 move。
	if p.CurrentAnim != nil && !p.CurrentAnim.Playing {
		p.CurrentAnim.Current = 0
		p.CurrentAnim.CurrentLoop = 0
		p.CurrentAnim.Playing = true
		p.CurrentAnim.LastUpdate = time.Now()
	}
	dx := p.TargetX - p.X
	dy := p.TargetY - p.Y
	dist := math.Sqrt(float64(dx*dx + dy*dy))
	if dist < 1 {
		a.switchToIdle2()
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
		a.switchToIdle2()
	}
}

// DiscoverPets 扫描 assets 目录发现可用宠物。
// 合法宠物要求：目录下至少有一个含 *.gif 的状态子目录，
// 避免误放的空目录 / 备份目录进入切换菜单。
func (a *App) DiscoverPets() {
	root := assetsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	a.Pets = nil
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		petDir := filepath.Join(root, e.Name())
		sub, err := os.ReadDir(petDir)
		if err != nil {
			continue
		}
		valid := false
		for _, se := range sub {
			if !se.IsDir() {
				continue
			}
			gifs, _ := filepath.Glob(filepath.Join(petDir, se.Name(), "*.gif"))
			if len(gifs) > 0 {
				valid = true
				break
			}
		}
		if valid {
			a.Pets = append(a.Pets, e.Name())
		}
	}
}

// LoadResources 加载当前宠物的 GIF 动画和音频文件
// 资产结构: assets/{petName}/{state}/{name}.gif + {name}.mp3/wav
// 每个 GIF 独立为一个动画变体（如 idle1~4），播放时随机选取。
// 资源先加载到临时 map，全部成功后才提交（供 SwitchPet 回滚）。
func (a *App) LoadResources() error {
	p := a.Pet
	petDir := filepath.Join(assetsRoot(), p.Name)
	entries, err := os.ReadDir(petDir)
	if err != nil {
		return err
	}

	anims := make(map[string][]*Animator)
	feetOf := make(map[*Animator]int)
	sounds := make(map[string][]string)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		state := e.Name()
		stateDir := filepath.Join(petDir, state)

		gifs, _ := filepath.Glob(filepath.Join(stateDir, "*.gif"))
		for _, path := range gifs {
			if anim, err := loadGIF(path); err == nil {
				anims[state] = append(anims[state], anim)
				feetOf[anim] = animFeet(anim)
			} else {
				println("Failed to load", path, ":", err.Error())
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

	// 全局归一化：全部变体统一画布尺寸与脚线，避免状态/变体切换时精灵跳动
	baseW, baseH, baseFeet := 0, 0, 0
	for _, vs := range anims {
		for _, anim := range vs {
			if anim.Width > baseW {
				baseW = anim.Width
			}
			if anim.Height > baseH {
				baseH = anim.Height
			}
		}
	}
	for _, feet := range feetOf {
		if feet > baseFeet {
			baseFeet = feet
		}
	}
	for _, vs := range anims {
		for _, anim := range vs {
			anim.normalizeFrames(baseW, baseH, baseFeet-feetOf[anim])
		}
	}

	// 默认动画：优先 idle2（第二变体），否则 idle 首个；再否则取第一个；
	// 无任何动画视为加载失败
	var base *Animator
	if vs := anims["idle"]; len(vs) > 0 {
		base = vs[0]
		if len(vs) > 1 {
			base = vs[1]
		}
	} else {
		for _, vs := range anims {
			if len(vs) > 0 {
				base = vs[0]
				break
			}
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
	variants := 0
	for _, vs := range anims {
		variants += len(vs)
	}
	println("loaded", p.Name, "states:", len(anims), "variants:", variants,
		"sounds:", len(sounds),
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

	// 取消在途 playHop（防止跳跃继续作用到新宠物）并清空音频队列
	// （防止旧宠物的音效在新宠物加载后才播放）。
	a.hopToken++
	speakerClear()

	oldAnims := p.Anims
	oldSounds := p.Sounds
	oldCurrent := p.CurrentAnim
	oldName := p.Name

	p.Anims = make(map[string][]*Animator)
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

	// 立即按新尺寸重渲染，避免窗口已变而分层表面仍是旧内容
	a.render()

	return nil
}

// renderDbg 记录渲染早退点是否已打印过（仅主线程访问）
var renderDbg = struct{ guard, frame, dc, dib bool }{}

// 渲染缓存（仅主线程访问）：复用 memDC/DIB，避免每帧分配 GDI 对象
// 泄漏导致对象耗尽后 CreateDIBSection 失败而画面卡死。
var (
	renderMemDC      uintptr
	renderBmp        uintptr
	renderBits       unsafe.Pointer
	renderW, renderH int32
)

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

	anim := p.CurrentAnim
	if anim.Current >= len(anim.Frames) || len(anim.Frames) == 0 {
		if !renderDbg.frame {
			renderDbg.frame = true
			println("render skip: frame nil, current:", anim.Current, "len:", len(anim.Frames))
		}
		return
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		if !renderDbg.dc {
			renderDbg.dc = true
			println("GetDC failed, err:", getLastError())
		}
		return
	}
	defer procReleaseDC.Call(0, screenDC)

	// 尺寸变化（缩放/切换宠物）时重建 DIB 与 DC
	if renderMemDC == 0 || renderW != p.Width || renderH != p.Height {
		a.rebuildRenderTarget(screenDC, p.Width, p.Height)
		if renderMemDC == 0 {
			return
		}
	}
	// 重建失败时保留旧 target；尺寸不匹配则直接返回（下一帧再试），
	// 避免用旧尺寸 DIB 按新尺寸读写造成越界或内容错乱。
	if renderW != p.Width || renderH != p.Height {
		return
	}

	// 命中 scaled 缓存时，render 退化为单次 copy（DIB 自顶向下、BGRA 与缓存布局一致）
	anim.ensureScaled(int(p.Width), int(p.Height))
	if anim.Current >= len(anim.scaled) {
		return
	}
	pixels := unsafe.Slice((*byte)(renderBits), int(p.Width)*int(p.Height)*4)
	copy(pixels, anim.scaled[anim.Current])

	blend := BLENDFUNCTION{AC_SRC_OVER, 0, 255, AC_SRC_ALPHA}
	src := POINT{0, 0}
	dst := POINT{p.X, p.Y}
	size := POINT{p.Width, p.Height}

	ret, _, _ := procUpdateLayeredWindow.Call(
		p.Hwnd, screenDC,
		uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		renderMemDC, uintptr(unsafe.Pointer(&src)),
		0, uintptr(unsafe.Pointer(&blend)), ULW_ALPHA,
	)
	if ret == 0 {
		println("UpdateLayeredWindow failed")
	} else if !a.RenderedOnce {
		a.RenderedOnce = true
		procShowWindow.Call(p.Hwnd, SW_SHOWNOACTIVATE)
		println("first render ok at", p.X, ",", p.Y, "size:", p.Width, "x", p.Height)
	}
}

// rebuildRenderTarget 按窗口尺寸重建 memDC 与 DIB（仅主线程调用）。
// 先构建新 target，成功后再替换旧的；失败则保留旧 target 供下一帧重试，
// 避免“删除旧 DIB 后创建新 DIB 失败”导致渲染永久停摆。
func (a *App) rebuildRenderTarget(screenDC uintptr, w, h int32) {
	newDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if newDC == 0 {
		return
	}

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
			Width:    int32(w),
			Height:   -h, // 自顶向下
			Planes:   1,
			BitCount: 32,
		},
	}

	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(screenDC, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 {
		if !renderDbg.dib {
			renderDbg.dib = true
			println("CreateDIBSection failed, err:", getLastError())
		}
		procDeleteDC.Call(newDC)
		return
	}
	procSelectObject.Call(newDC, hbm)

	// 成功后再释放旧 target 并替换
	if renderMemDC != 0 {
		procDeleteDC.Call(renderMemDC)
	}
	if renderBmp != 0 {
		procDeleteObject.Call(renderBmp)
	}
	renderMemDC = newDC
	renderBmp = hbm
	renderBits = bits
	renderW = w
	renderH = h
}

func (a *App) resizeWindow(percent int) {
	a.Pet.SetScale(percent)
	// 缩放后窗口变大，把宠物重新钳制到屏幕/工作区内，避免被屏幕边缘裁掉
	a.Pet.X, a.Pet.Y = a.clampToScreen(a.Pet.X, a.Pet.Y)
	procSetWindowPos.Call(a.Pet.Hwnd, 0,
		uintptr(a.Pet.X), uintptr(a.Pet.Y),
		uintptr(a.Pet.Width), uintptr(a.Pet.Height),
		0x0004) // SWP_NOZORDER（位置已由 clamp 决定）
	a.Cfg.ScalePercent = percent
	a.saveConfig()
	// 立即按新尺寸重建 DIB 并重渲染分层表面。原先只依赖下一帧 WM_TIMER，
	// 且渲染受 Paused/Away 影响会被跳过，导致窗口尺寸已变、表面仍是旧尺寸
	// （“窗口变大但宠物未缩放，只看到原图一部分”）。此处同步刷新保证一致。
	a.render()
}
