package internal

import (
	"image"
	"image/draw"
	"image/gif"
	"os"
	"time"
)

// 动画帧
type Frame struct {
	Image *image.RGBA
	// BGRA 是源尺寸的预转换 BGRA 字节流（normalizeFrames 后填充）。
	// render 不再做每像素 RGBA→BGRA，直接从 BGRA 缩放到目标缓冲。
	BGRA  []byte
	Delay time.Duration
}

// 动画播放器。仅由主线程访问（switchAnim/Update/GetFrame/render），
// 不加锁；任何后台 goroutine 都不得持有 *Animator 后修改其字段。
type Animator struct {
	Frames      []Frame
	Current     int
	LoopCount   int
	CurrentLoop int
	LastUpdate  time.Time
	Playing     bool
	Width       int
	Height      int
	// scaled 是按当前 Pet.Width×Height 预缩放好的 BGRA 帧缓存；
	// 命中后 render 退化为单次 copy 到 DIB。尺寸变化时由 ensureScaled 重建。
	scaled  [][]byte
	scaledW int
	scaledH int
	// scaledMirror 是 scaled 的水平镜像缓存（宠物朝左行走时用），
	// 由 ensureScaledMirror 惰性构建，尺寸变化时一并失效。
	scaledMirror [][]byte
}

func (a *Animator) Update() {
	if !a.Playing || len(a.Frames) == 0 {
		return
	}

	now := time.Now()
	delay := a.Frames[a.Current].Delay

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
// 同时为每帧生成源尺寸 BGRA 字节流，render 仅做最近邻缩放 + memcpy。
func (a *Animator) normalizeFrames(w, h, oy int) {
	for i, fr := range a.Frames {
		padded := padFrame(fr, w, h, oy)
		padded.BGRA = rgbaToBGRA(padded.Image)
		a.Frames[i] = padded
	}
	a.Width = w
	a.Height = h
	a.scaled = nil
	a.scaledMirror = nil
	a.scaledW = 0
	a.scaledH = 0
}

// rgbaToBGRA 将 RGBA 帧转为 BGRA 字节流（Win32 DIB 期望的布局）。
func rgbaToBGRA(src *image.RGBA) []byte {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	out := make([]byte, w*h*4)
	stride := src.Stride
	for y := 0; y < h; y++ {
		row := src.Pix[y*stride : y*stride+w*4]
		for x := 0; x < w; x++ {
			si := x * 4
			di := (y*w + x) * 4
			out[di] = row[si+2]   // B
			out[di+1] = row[si+1] // G
			out[di+2] = row[si]   // R
			out[di+3] = row[si+3] // A
		}
	}
	return out
}

// ensureScaled 按 dstW×dstH 重建预缩放 BGRA 帧缓存。仅在尺寸变化时重建，
// 命中后 render 退化为单次 copy。内存换 CPU：典型宠物缓存约几 MB~几十 MB。
func (a *Animator) ensureScaled(dstW, dstH int) {
	if a.scaled != nil && a.scaledW == dstW && a.scaledH == dstH {
		return
	}
	a.scaled = make([][]byte, len(a.Frames))
	a.scaledMirror = nil // 尺寸变化时镜像缓存一并失效
	srcW := a.Width
	srcH := a.Height
	for i := range a.Frames {
		src := a.Frames[i].BGRA
		out := make([]byte, dstW*dstH*4)
		for dy := 0; dy < dstH; dy++ {
			sy := dy * srcH / dstH
			for dx := 0; dx < dstW; dx++ {
				sx := dx * srcW / dstW
				si := (sy*srcW + sx) * 4
				di := (dy*dstW + dx) * 4
				out[di] = src[si]
				out[di+1] = src[si+1]
				out[di+2] = src[si+2]
				out[di+3] = src[si+3]
			}
		}
		a.scaled[i] = out
	}
	a.scaledW = dstW
	a.scaledH = dstH
}

// ensureScaledMirror 按 dstW×dstH 重建水平镜像的预缩放 BGRA 帧缓存
// （宠物朝左行走时使用）。与 ensureScaled 仅采样方向相反，惰性构建：
// 只有实际朝左移动时才占一份额外缓存。
func (a *Animator) ensureScaledMirror(dstW, dstH int) {
	if a.scaledMirror != nil && a.scaledW == dstW && a.scaledH == dstH {
		return
	}
	a.scaledMirror = make([][]byte, len(a.Frames))
	srcW := a.Width
	srcH := a.Height
	for i := range a.Frames {
		src := a.Frames[i].BGRA
		out := make([]byte, dstW*dstH*4)
		for dy := 0; dy < dstH; dy++ {
			sy := dy * srcH / dstH
			for dx := 0; dx < dstW; dx++ {
				sx := (dstW - 1 - dx) * srcW / dstW
				si := (sy*srcW + sx) * 4
				di := (dy*dstW + dx) * 4
				out[di] = src[si]
				out[di+1] = src[si+1]
				out[di+2] = src[si+2]
				out[di+3] = src[si+3]
			}
		}
		a.scaledMirror[i] = out
	}
}
