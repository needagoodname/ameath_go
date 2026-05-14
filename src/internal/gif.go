package internal

import (
	"image"
	"image/gif"
	"image/draw"

	"sync"
	"os"
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
	}
	
	// 获取画布尺寸
	width := g.Config.Width
	height := g.Config.Height
	pet.Width = int32(width)
	pet.Height = int32(height)
	
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