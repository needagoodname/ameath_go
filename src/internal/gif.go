package internal

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"os"
	"sync"
	"time"
)

// 动画帧
type Frame struct {
	Image *image.RGBA
	Delay time.Duration
}

// 动画播放器
type Animator struct {
	Frames      []Frame
	Current     int
	LoopCount   int
	CurrentLoop int
	LastUpdate  time.Time
	Playing     bool
	Mutex       sync.RWMutex
	Width       int
	Height      int
}

func (a *Animator) Update() {
	if !a.Playing || len(a.Frames) == 0 {
		return
	}

	now := time.Now()
	delay := a.Frames[a.Current].Delay

	a.Mutex.Lock()
	defer a.Mutex.Unlock()

	if now.Sub(a.LastUpdate) < delay {
		return
	}

	a.Current++
	if a.Current >= len(a.Frames) {
		a.Current = 0
		a.CurrentLoop++
		if a.LoopCount > 0 && a.CurrentLoop >= a.LoopCount {
			a.Playing = false
		}
	}
	a.LastUpdate = now
}

func (a *Animator) GetFrame() *image.RGBA {
	a.Mutex.RLock()
	defer a.Mutex.RUnlock()
	if a.Current < len(a.Frames) {
		return a.Frames[a.Current].Image
	}
	return nil
}

// 加载 GIF 文件
func loadGIF(path string) (*Animator, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, err
	}

	anim := &Animator{
		LoopCount: g.LoopCount,
		Frames:    make([]Frame, len(g.Image)),
		Width:     g.Config.Width,
		Height:    g.Config.Height,
	}

	// 获取画布尺寸
	width := g.Config.Width
	height := g.Config.Height

	// 处理每一帧
	var prevFrame *image.RGBA

	for i, srcImg := range g.Image {
		bounds := srcImg.Bounds()

		// 创建 RGBA 画布
		rgba := image.NewRGBA(image.Rect(0, 0, width, height))

		// 处理 GIF 的 disposal 方法
		if i > 0 && g.Disposal[i-1] != gif.DisposalNone && prevFrame != nil {
			// 复制上一帧作为基础
			draw.Draw(rgba, rgba.Bounds(), prevFrame, image.Point{}, draw.Src)
		}

		// 绘制当前帧
		draw.Draw(rgba, bounds, srcImg, bounds.Min, draw.Over)

		// 保存为前一帧
		prevFrame = image.NewRGBA(rgba.Bounds())
		draw.Draw(prevFrame, prevFrame.Bounds(), rgba, image.Point{}, draw.Src)

		// 帧延迟（GIF 单位是 1/100 秒）
		delay := time.Duration(g.Delay[i]) * 10 * time.Millisecond
		if delay < 20*time.Millisecond {
			delay = 100 * time.Millisecond
		}

		anim.Frames[i] = Frame{
			Image: rgba,
			Delay: delay,
		}
	}

	return anim, nil
}

// loadGIFs 加载目录下全部 GIF 并合并为单个 Animator：
// 帧序列按文件名顺序拼接，画布取最大尺寸，各 GIF 按脚线对齐。
// 返回合并后的 Animator 及其脚线（全部帧非透明底边的最大行索引）。
func loadGIFs(paths []string) (*Animator, int, error) {
	anims := make([]*Animator, 0, len(paths))
	feet := make([]int, 0, len(paths))
	maxW, maxH, maxFeet := 0, 0, 0
	for _, p := range paths {
		anim, err := loadGIF(p)
		if err != nil {
			println("Failed to load", p, ":", err.Error())
			continue
		}
		anims = append(anims, anim)
		f := animFeet(anim)
		feet = append(feet, f)
		if anim.Width > maxW {
			maxW = anim.Width
		}
		if anim.Height > maxH {
			maxH = anim.Height
		}
		if f > maxFeet {
			maxFeet = f
		}
	}
	if len(anims) == 0 {
		return nil, 0, fmt.Errorf("no valid gif")
	}

	merged := &Animator{
		LoopCount: mergedLoopCount(anims),
		Width:     maxW,
		Height:    maxH,
	}
	for i, anim := range anims {
		oy := maxFeet - feet[i]
		for _, fr := range anim.Frames {
			merged.Frames = append(merged.Frames, padFrame(fr, maxW, maxH, oy))
		}
	}
	return merged, maxFeet, nil
}

// animFeet 返回动画全部帧的非透明底边最大值（0-based 行索引，脚线）。
func animFeet(a *Animator) int {
	maxBottom := 0
	for _, fr := range a.Frames {
		b := fr.Image.Bounds()
		found := false
		for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
			for x := b.Min.X; x < b.Max.X; x++ {
				if fr.Image.RGBAAt(x, y).A > 32 {
					if y > maxBottom {
						maxBottom = y
					}
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	return maxBottom
}

// mergedLoopCount 任一源 GIF 无限循环则整体无限，否则取最大循环数。
func mergedLoopCount(anims []*Animator) int {
	n := 0
	for _, a := range anims {
		if a.LoopCount == 0 {
			return 0
		}
		if a.LoopCount > n {
			n = a.LoopCount
		}
	}
	return n
}

// padFrame 将帧平铺到 w×h 画布，内容整体下移 oy（超出画布部分裁剪）。
func padFrame(fr Frame, w, h, oy int) Frame {
	b := fr.Image.Bounds()
	if b.Dx() == w && b.Dy() == h && oy == 0 {
		return fr
	}
	padded := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(padded, b.Add(image.Pt(0, oy)), fr.Image, image.Point{}, draw.Src)
	return Frame{Image: padded, Delay: fr.Delay}
}

// normalizeFrames 将所有帧平铺到 w×h 画布并下移 oy（跨状态脚线对齐用）。
func (a *Animator) normalizeFrames(w, h, oy int) {
	for i, fr := range a.Frames {
		a.Frames[i] = padFrame(fr, w, h, oy)
	}
	a.Width = w
	a.Height = h
}
