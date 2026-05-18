# 托盘缩放功能实现计划

## Context

在当前桌面宠物应用中，宠物角色以 GIF 原始尺寸（通常 128×128）1:1 渲染，无法调整大小。需要在系统托盘中添加"缩放"子菜单，允许用户按预设比例（50%, 75%, 100%, 125%, 150%, 200%）调整宠物显示大小，并将选择持久化到配置文件。

## 受影响文件

| 文件 | 改动类型 |
|---|---|
| `src/internal/config.go` | 添加 `ScalePercent` 字段 |
| `src/internal/pet.go` | 添加 `BaseWidth`/`BaseHeight` 字段 + `SetScale` 方法 |
| `src/internal/gif.go` | 修改 `loadGIF()` 存储原始尺寸并应用缩放 |
| `src/internal/action.go` | 修改 `render()` 实现最近邻缩放；添加 `resizeWindow()` |
| `src/internal/ui.go` | 添加"缩放"子菜单及事件处理 |
| `src/internal/windows.go` | 无需改动 |
| `src/main.go` | 无需改动 |

## 详细改动

### 1. config.go — 添加 ScalePercent 字段

```go
type Config struct {
    AudioOn      bool   `json:"audio_on"`
    AutoStart    bool   `json:"auto_start"`
    CurrentPet   string `json:"current_pet"`
    WindowX      int32  `json:"window_x"`
    WindowY      int32  `json:"window_y"`
    ScalePercent int    `json:"scale_percent"`  // 新增
}
```

`LoadConfig()` 中 `json.Unmarshal` 之后添加向后兼容处理：
```go
if Cfg.ScalePercent <= 0 {
    Cfg.ScalePercent = 100
}
```

### 2. pet.go — 添加原始尺寸和缩放方法

```go
type Pet struct {
    // ... 现有字段 ...
    Width      int32
    Height     int32
    BaseWidth  int32  // 新增：GIF 原始宽度
    BaseHeight int32  // 新增：GIF 原始高度
    // ... 其余字段不变 ...
}

func (p *Pet) SetScale(percent int) {
    p.Width = int32(float64(p.BaseWidth) * float64(percent) / 100.0)
    p.Height = int32(float64(p.BaseHeight) * float64(percent) / 100.0)
}
```

### 3. gif.go — loadGIF 存储原始尺寸

将 `loadGIF()` 中的：
```go
pet.Width = int32(width)
pet.Height = int32(height)
```
替换为：
```go
pet.BaseWidth = int32(width)
pet.BaseHeight = int32(height)
pet.SetScale(Cfg.ScalePercent)
```

### 4. action.go — 缩放渲染 + resizeWindow

**render() 像素循环**（替换第 232-243 行）：

```go
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
```

DIB 头中的 `Width`/`Height` 已使用 `pet.Width`/`pet.Height`（缩放后的值），无需改动。`UpdateLayeredWindow` 的 `size` 同理。

**新增 resizeWindow 函数**（追加到 action.go）：

```go
func resizeWindow(percent int) {
    pet.SetScale(percent)
    procSetWindowPos.Call(pet.Hwnd, 0, 0, 0,
        uintptr(pet.Width), uintptr(pet.Height),
        0x0002|0x0004) // SWP_NOMOVE | SWP_NOZORDER
    Cfg.ScalePercent = percent
    saveConfig()
}
```

**SwitchPet 中追加窗口大小更新**（LoadResources 调用之后）：
```go
LoadResources(petName)
procSetWindowPos.Call(pet.Hwnd, 0, 0, 0,
    uintptr(pet.Width), uintptr(pet.Height),
    0x0002|0x0004)
```

### 5. ui.go — 缩放子菜单

在 "Switch Pet" 子菜单块之后、分隔符之前，添加：

```go
// 缩放子菜单
mScale := systray.AddMenuItem("Scale", "")
scalePresets := []int{50, 75, 100, 125, 150, 200}
scaleMenuItems := make(map[int]*systray.MenuItem)
for _, pct := range scalePresets {
    title := fmt.Sprintf("%d%%", pct)
    if Cfg.ScalePercent == pct {
        title = "✓ " + title
    }
    item := mScale.AddSubMenuItem(title, "")
    scaleMenuItems[pct] = item
}
```

在 Switch Pet 子菜单 goroutine 块之后，添加事件处理：

```go
for pct, item := range scaleMenuItems {
    go func(percent int, mi *systray.MenuItem) {
        for range mi.ClickedCh {
            if percent != Cfg.ScalePercent {
                resizeWindow(percent)
                for sp, smi := range scaleMenuItems {
                    if sp == percent {
                        smi.SetTitle(fmt.Sprintf("✓ %d%%", sp))
                    } else {
                        smi.SetTitle(fmt.Sprintf("%d%%", sp))
                    }
                }
            }
        }
    }(pct, item)
}
```

需要在 `ui.go` 的 import 中添加 `"fmt"`。

## 数据流

```
启动:
  LoadConfig() → ScalePercent = 100（或已保存值）
  NewPet() → Width/Height = 128（占位）
  LoadResources() → loadGIF() → BaseWidth/BaseHeight 赋值 → SetScale() → Width/Height = 缩放值
  RunMessageLoop() → CreateWindowExW(W=pet.Width, H=pet.Height) ✓ 已是缩放后尺寸

缩放变更:
  点击菜单项 → resizeWindow(percent)
    → SetScale(percent) → 从 BaseWidth/BaseHeight 重新计算 Width/Height
    → SetWindowPos(SWP_NOMOVE|SWP_NOZORDER) → 原地调整窗口大小
    → saveConfig() → 持久化 ScalePercent

渲染:
  GetFrame() → *image.RGBA（BaseWidth × BaseHeight，原始尺寸）
  DIB section → Width × Height（缩放尺寸）
  最近邻采样 → 从源像素映射到目标像素
  UpdateLayeredWindow(W=Width, H=Height)
```

## 验证方法

1. `cd src && go build -o ../ameath.exe .` — 确认编译通过
2. 运行应用，检查托盘菜单中出现 "Scale" 子菜单
3. 点击不同比例（50%/75%/100%/125%/150%/200%），观察宠物大小即时变化
4. 重启应用，确认缩放比例被记住（检查 config.json 中 `scale_percent` 字段）
5. 拖拽放大后的宠物，确认拖拽和边界限制正常工作
6. 切换宠物（Switch Pet），确认新宠物使用当前缩放比例
