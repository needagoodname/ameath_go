package internal

import (
	"os"
	"path/filepath"
	"time"
	"unsafe"
	
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
)

var (
	assetsDir = "./assets"
)

// 切换动画
func switchAnim(state string) {
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

// 加载资源
func loadResources(pet *Pet) {
	states := []string{"idle", "walk", "sleep", "eat", "happy", "click"}
	
	for _, s := range states {
		// 加载 GIF
		gifPath := filepath.Join(assetsDir, s+".gif")
		if anim, err := loadGIF(gifPath); err == nil {
			pet.Anims[s] = anim
		} else {
			println("Failed to load", gifPath, ":", err.Error())
		}
		
		// 加载音频
		for _, ext := range []string{".mp3", ".wav"} {
			sndPath := filepath.Join(assetsDir, s+ext)
			if _, err := os.Stat(sndPath); err == nil {
				pet.Sounds[s] = sndPath
				break
			}
		}
	}
	
	// 默认使用 idle
	if pet.Anims["idle"] != nil {
		pet.CurrentAnim = pet.Anims["idle"]
		pet.CurrentAnim.Playing = true
		pet.CurrentAnim.LastUpdate = time.Now()
	}
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
	
	// 复制像素 BGRA
	pixels := (*[1 << 20]byte)(bits)
	for y := 0; y < int(pet.Height); y++ {
		for x := 0; x < int(pet.Width); x++ {
			idx := (y*int(pet.Width) + x) * 4
			c := frame.RGBAAt(x, y)
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