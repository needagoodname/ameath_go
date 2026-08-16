package internal

import (
	"encoding/binary"
	"image"
	"os"
	"path/filepath"
)

const trayIconSize = 32

// LoadTrayIcon 生成托盘图标字节并存入 a.TrayIcon（启动期调用，systray 前）：
// 优先 assets/{petName}/icon.ico 与 assets/icon.ico，
// 否则用 idle 首个变体的首帧缩放生成 32×32 ICO。
func (a *App) LoadTrayIcon() {
	p := a.Pet
	for _, path := range []string{
		filepath.Join(assetsRoot(), p.Name, "icon.ico"),
		filepath.Join(assetsRoot(), "icon.ico"),
	} {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			a.TrayIcon = b
			println("tray icon from", path)
			return
		}
	}

	var first *image.RGBA
	if vs := p.Anims["idle"]; len(vs) > 0 && len(vs[0].Frames) > 0 {
		first = vs[0].Frames[0].Image
	} else {
		for _, vs := range p.Anims {
			if len(vs) > 0 && len(vs[0].Frames) > 0 {
				first = vs[0].Frames[0].Image
				break
			}
		}
	}
	if first == nil {
		println("tray icon: no frame available")
		return
	}
	a.TrayIcon = frameToICO(first, trayIconSize)
	println("tray icon generated from first frame")
}

// frameToICO 将 RGBA 帧最近邻缩放到 size×size 并编码为 ICO：
// 32bpp BGRA 位图 + 全 0 AND 掩码（透明由 alpha 通道表达），行序自底向上。
func frameToICO(src *image.RGBA, size int) []byte {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	xor := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		sy := (size - 1 - y) * sh / size
		for x := 0; x < size; x++ {
			sx := x * sw / size
			c := src.RGBAAt(sx, sy)
			i := (y*size + x) * 4
			xor[i], xor[i+1], xor[i+2], xor[i+3] = c.B, c.G, c.R, c.A
		}
	}
	andRow := (size + 31) / 32 * 4
	and := make([]byte, andRow*size)

	buf := make([]byte, 0, 22+40+len(xor)+len(and))
	putU16 := func(v uint16) { buf = binary.LittleEndian.AppendUint16(buf, v) }
	putU32 := func(v uint32) { buf = binary.LittleEndian.AppendUint32(buf, v) }

	// ICONDIR + ICONDIRENTRY
	putU16(0) // reserved
	putU16(1) // type: icon
	putU16(1) // count
	buf = append(buf, byte(size), byte(size), 0, 0)
	putU16(1)  // planes
	putU16(32) // bitcount
	putU32(uint32(40 + len(xor) + len(and)))
	putU32(22) // image offset

	// BITMAPINFOHEADER（高度为 XOR+AND 双倍）
	putU32(40)
	putU32(uint32(size))
	putU32(uint32(size * 2))
	putU16(1) // planes
	putU16(32)
	putU32(0) // BI_RGB
	putU32(uint32(len(xor) + len(and)))
	putU32(0) // XPelsPerMeter
	putU32(0) // YPelsPerMeter
	putU32(0) // ClrUsed
	putU32(0) // ClrImportant

	buf = append(buf, xor...)
	buf = append(buf, and...)
	return buf
}
