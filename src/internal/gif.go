package internal

import (
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
