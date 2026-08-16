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

	// 合成每一帧：维护持久画布，按 GIF89a disposal 规范处理，
	// 避免上一帧内容残留到下一帧造成残影。
	//   DisposalBackground → 下一帧前清空为透明
	//   DisposalPrevious  → 下一帧前恢复为上一帧绘制前的快照
	//   DisposalNone      → 保留画布
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	snapshots := make([]*image.RGBA, len(g.Image))

	for i, srcImg := range g.Image {
		bounds := srcImg.Bounds()

		if i > 0 {
			switch g.Disposal[i-1] {
			case gif.DisposalBackground:
				draw.Draw(canvas, canvas.Bounds(), image.Transparent, image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				if snapshots[i-1] != nil {
					draw.Draw(canvas, canvas.Bounds(), snapshots[i-1], image.Point{}, draw.Src)
				}
			}
		}

		// 记录绘制前的画布快照，供 DisposalPrevious 恢复
		snapshots[i] = image.NewRGBA(canvas.Bounds())
		draw.Draw(snapshots[i], snapshots[i].Bounds(), canvas, image.Point{}, draw.Src)

		// 绘制当前帧
		draw.Draw(canvas, bounds, srcImg, bounds.Min, draw.Over)

		// 当前帧独立快照，避免后续帧污染本帧
		frame := image.NewRGBA(canvas.Bounds())
		draw.Draw(frame, frame.Bounds(), canvas, image.Point{}, draw.Src)

		// 帧延迟（GIF 单位是 1/100 秒）
		delay := time.Duration(g.Delay[i]) * 10 * time.Millisecond
		if delay < 20*time.Millisecond {
			delay = 100 * time.Millisecond
		}

		anim.Frames[i] = Frame{
			Image: frame,
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
