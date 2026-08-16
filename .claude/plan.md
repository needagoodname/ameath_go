# 托盘菜单动态化重构

## Context

当前 `ui.go` 中硬编码了 4 个行为菜单项（喂食/玩耍/睡觉/叫醒），分别映射到 `eat`/`happy`/`sleep`/`idle`。但动画状态是由 `./assets/{petName}/{state}/` 目录名动态发现的，二者脱节：宠物缺少某状态时菜单项无效，宠物有额外状态时菜单无入口。

改为：行为菜单项由 `pet.Anims` 的 key 动态生成，目录名即菜单标签，排除内部状态。

## 修改文件

### 1. `src/internal/action.go`

**新增隐藏状态集合：**

```go
var internalStates = map[string]bool{
    "walk":  true,
    "click": true,
}
```

**新增函数 `MenuStates()`**，返回所有可用宠物的菜单可见状态合集（用于托盘初始化时一次性创建所有菜单项）：

```go
func MenuStates() []string {
    set := map[string]bool{}
    for _, name := range availablePets {
        dir := filepath.Join(".", "assets", name)
        entries, _ := os.ReadDir(dir)
        for _, e := range entries {
            if e.IsDir() && !internalStates[e.Name()] {
                set[e.Name()] = true
            }
        }
    }
    out := make([]string, 0, len(set))
    for s := range set { out = append(out, s) }
    sort.Strings(out)
    return out
}
```

**新增函数 `HasMenuState(state string) bool`**，供托盘在切换宠物时判断某个菜单项是否应显示：

```go
func HasMenuState(state string) bool {
    _, ok := pet.Anims[state]
    return ok && !internalStates[state]
}
```

需要添加 `"sort"` import。

### 2. `src/internal/ui.go`

**删除**硬编码的菜单项创建（mFeed/mPlay/mSleep/mWake 及其上方分隔符）。

**新增**动态创建行为菜单项的逻辑：

```go
stateMenuItems := make(map[string]*systray.MenuItem)
for _, state := range MenuStates() {
    item := systray.AddMenuItem(state, "")
    stateMenuItems[state] = item
}
```

**重写事件循环**：将原来 4 个独立的 case 合并为统一处理，每个动态菜单项启动一个 goroutine 监听点击。`happy` 状态的跳跃动画通过 `s == "happy"` 保留。

**SwitchPet goroutine 中**：切换宠物后遍历 `stateMenuItems`，根据 `HasMenuState()` 调用 `mi.Show()` / `mi.Hide()` 更新可见性。

## 特殊行为保留

`happy` 状态的跳跃动画通过字符串判断保留，行为不变。

## 验证方式

1. `cd src && go build -o ../ameath.exe .` 确保编译通过
2. 菜单项数量与资产目录中非内部状态目录数一致
3. 切换宠物时菜单项正确显示/隐藏

---

# 架构重构：App 结构体化 + 主线程事件收敛 + 功能 Bug 修复

## Context

架构审视发现两类问题：

1. **并发模型错误**：包级全局状态（`pet`、`audioOn`、`paused`、`away`、`Cfg`）被主线程消息循环与托盘 goroutine（ui.go 每菜单项一个）无同步地并发读写；`Animator` 的锁在 `switchAnim` 重置字段时被绕过。按 Go 内存模型属未定义行为。
2. **全局单例耦合**：`loadGIF()` 解码器反向写全局 `pet`；视图逻辑（happy 跳跃）硬编码在 ui.go；退出路径三机制并存且 `close(quitChan)` 有竞态。

方案：**引入 `App` 结构体收纳全部状态 + 托盘命令经 `PostMessage` 收归主线程执行**（Win32 单线程事件模型），并一并修复功能 bug（移动速度、屏幕边界/多显示器/DPI、资源路径、遮挡误判、退出保存位置）。

范围确认：**本次不做跨平台**。保持 Windows-only 实现，不引入 Platform 接口边界；App 结构体化与事件收敛模型本身已为将来跨平台铺路（命令通道概念平台中立），届时再以接口化窗口层追加。

## 设计总览

### 线程所有权模型（核心规则）

```
托盘 goroutine（若干）          主线程（消息循环/wndProc/定时器）
  ├─ 读 ClickedCh            ├─ 唯一允许读写 App 状态
  ├─ systray.MenuItem.*      ├─ 执行 WM_TIMER：动画/移动/AI/渲染/遮挡
  ├─ postCmd(f) ──通道──▶   ├─ 执行 WM_APP_EXEC_CMD：drain cmdChan 运行闭包
  └─ PostMessage(WM_APP_QUIT)└─ WM_DESTROY：KillTimer/存配置/PostQuitMessage
```

- 所有状态变更（切动画、切宠物、缩放、暂停、静音、显隐、happy 跳跃、退出）只能以「闭包投递」形式在主线程执行。
- 托盘 goroutine 只做两件事：读 `ClickedCh`、调 `app.postCmd(f)`。
- `playSound` 的解码 goroutine 只持有局部变量副本。
- 菜单构建期（OnTrayReady）只读启动时已就绪的字段，无并发写者。

### 命令通道

```go
type App struct {
    Pet      *Pet
    Cfg      Config
    AudioOn  bool
    Paused   bool
    Away     bool
    Pets     []string
    ScreenW  int32
    ScreenH  int32
    cmdChan  chan func() // 缓冲 64
}

var app *App // wndProc 回调无法带 receiver，经此访问

func (a *App) postCmd(f func()) {
    select {
    case a.cmdChan <- f:
        procPostMessage.Call(a.Pet.Hwnd, WM_APP_EXEC_CMD, 0, 0)
    default:
        println("command queue full")
    }
}
```

wndProc 新增：`WM_APP_EXEC_CMD`（drain cmdChan 执行闭包）、`WM_APP_QUIT`（DestroyWindow）。

### 退出路径（单一路径，删除 quitChan）

托盘退出点击 → `PostMessage(WM_APP_QUIT)` → `DestroyWindow` → `WM_DESTROY`（KillTimer×3、保存位置与配置、PostQuitMessage）→ GetMessage 返回 0 → `RunMessageLoop` 返回 → main 调用 `systray.Quit()` → 进程退出。

### 启动顺序（保证 hwnd 先于托盘存在）

NewApp → LoadConfig → DiscoverPets → NewPet → InitAudio → LoadResources → SyncAutoStart → **CreateWindow()**（从 RunMessageLoop 拆出）→ `go systray.Run(...)` → RunMessageLoop() → `systray.Quit()`。`SetProcessDPIAware()` 在 CreateWindow 前调用。

## 修改文件

### 1. `src/internal/state.go` → 删除，新建 `src/internal/app.go`

App 结构体 + `NewApp()`（cmdChan 缓冲 64）+ `postCmd` + `moveBy(dx, dy)`（SetWindowPos 相对移动）+ `playHop()`（happy 跳跃：goroutine 每 100ms 交替投递 moveBy(0,-20)/(0,+20)，共 3 次，保持原视觉节奏）+ `clampToScreen(x, y)`。

### 2. `src/internal/windows.go`

- 拆 `RunMessageLoop` 为 `CreateWindow()` + `RunMessageLoop()`。
- 新增 proc：`GetSystemMetrics`、`SetProcessDPIAware`、`DestroyWindow`；屏幕尺寸存入 `app.ScreenW/ScreenH`。
- 新增常量 `WM_APP_EXEC_CMD`、`WM_APP_QUIT`。
- WM_TIMER(1)：`Update()` 后新增 `app.updateMovement()` 再 `render()`。
- **lParam 符号修复**：`x := int32(int16(lParam & 0xFFFF))`（WM_MOUSEMOVE 与 WM_LBUTTONDOWN 两处）。
- WM_DESTROY：KillTimer×3 → 保存窗口位置 → `app.saveConfig()` → PostQuitMessage。
- `checkOcclusion` alpha 感知：采样中心 + 四分之一处 5 点，经 `GetFrame()` 精灵坐标换算检查 alpha ≥ 32 才参与 `WindowFromPoint` 命中判断，修透明像素误判。

### 3. `src/internal/gif.go`

- `loadGIF(path) (*Animator, error)`：删除对全局 pet/Cfg 的副作用；`Animator` 增加 `Width, Height int`。

### 4. `src/internal/action.go`（大部分函数改为 App 方法）

- `./assets` → `assetsRoot()`（exe 同目录，复用 autostart.go 的 `exePath()`，失败回退 `./assets`）。
- `LoadResources`：加载完成后取 idle 动画（否则第一个）的尺寸设 `BaseWidth/BaseHeight` 并 `SetScale`。
- **`SwitchPet` 失败回滚**：先向临时 map 加载，全部成功才原子替换，失败原状态不动。
- **AI 与移动分离**：`updateAI()`（2s tick）只管状态转移（目标生成用 `clampToScreen`）；新增 `updateMovement()`（60fps tick，~2px/帧 ≈ 120px/s），删除自定义 `sqrt`。

### 5. `src/internal/ui.go`

- 所有菜单 goroutine 的 handler 改为 `app.postCmd(func(){ ... })`；菜单标题更新移入闭包。
- happy 分支：闭包内 `app.switchAnim(s); if s == "happy" { app.playHop() }`，删除 time.Sleep 跳跃循环。
- 退出：`procPostMessage.Call(app.Pet.Hwnd, WM_APP_QUIT, 0, 0)`。

### 6. `src/internal/audio.go` / `config.go` / `autostart.go`

`InitAudio`/`playSound` 改 App 方法（playSound 在主线程取值，goroutine 内仅用局部副本）；`Cfg` 移入 App，`LoadConfig`/`saveConfig` 改方法；`SyncAutoStart` 改方法。

### 7. `src/main.go`

按上文启动顺序重写。

### 8. 清理

- 删除死代码文件 `typeError.go`、`typeLog.go`。
- `go.mod`：删除 `replace github.com/na_me/ameath-go => ./` 自引用。
- 更新 `CLAUDE.md` 架构描述与 Known issues。

## 验证方式

本机（Linux）无 Go 工具链，需用户在 Windows 侧执行：

1. `cd src && go mod tidy && go build -o ../ameath.exe .`
2. 建议 `go build -race` 跑 5 分钟覆盖托盘操作，确认无竞态报告
3. 手动清单：行走速度正常（~120px/s）；happy 跳三下节奏不变；托盘全部菜单项生效；拖到副屏不跳变；退出后位置持久化；从其他工作目录启动资源正常；遮挡冻结/恢复正常；切换 GIF 缺失宠物报错且原宠物不受影响

---

# 资源组织 + 状态多 GIF 合并

## Context

共享文件夹（~/下载/share_folder/ameath-go）中资源组织进 `assets/`：idle1~4.gif → idle、move.gif → walk、drag.gif → click。四个 idle 变体需同时生效，而当前加载器每状态只取一个 GIF；且 idle1 画布 200×193、其余 200×200，脚线（非透明底边）不一（192/187/190/199），直接合并会跳 5~12px。语音文件由用户手动组织（本次不动 voice/）。

方案：**状态目录内全部 GIF 合并为单个 Animator，帧按文件名顺序拼接，画布取最大尺寸，各 GIF 按脚线对齐；跨状态再全局归一化一次（统一画布与脚线）**。

## 修改文件

### 1. `src/internal/gif.go`

- 新增 `loadGIFs(paths) (*Animator, feet int, err error)`：逐个 `loadGIF`（失败跳过并打印），合并帧序列，`padFrame` 按 `oy = maxFeet - gifFeet` 平铺到最大画布。
- 新增 `animFeet(anim) int`：全部帧非透明底边最大行索引（alpha > 32）。
- 新增 `mergedLoopCount`：任一源无限循环则整体无限，否则取最大。
- 新增 `padFrame(fr, w, h, oy)`：帧平铺到 w×h 画布并下移 oy（超出裁剪；无变化时原样返回）。
- 新增 `Animator.normalizeFrames(w, h, oy)`：全部帧统一画布。

### 2. `src/internal/action.go`

`LoadResources`：每状态目录用 `loadGIFs` 加载全部 GIF；加载完成后全局归一化（baseW/baseH = 各状态最大画布，baseFeet = 最大脚线，各状态 `normalizeFrames(baseW, baseH, baseFeet-feetOf[state])`），`BaseWidth/BaseHeight` 取全局画布。

### 3. 资源目录

```
assets/ameath/idle/{idle1..4}.gif
assets/ameath/walk/move.gif
assets/ameath/click/drag.gif
```

ameath.gif（1000×1000）、ameath.ico、ameath_content.png、screen1-7.gif（截图）不入库，留共享文件夹。

## 验证方式

1. 编译：`cd src && go build -o ../ameath.exe .`
2. idle 状态依次轮播 4 个变体（约 2.7s/个），变体间无垂直跳动
3. idle ↔ walk ↔ click 切换无跳动；拖动时 drag 动画正常
4. 语音文件由用户放入对应状态目录后，状态切换时随机播放

---

# 托盘图标接入 + idle 多变体随机播放 + 菜单 GIF 过滤

## Context

用户反馈：1) 托盘无图标只是空白；2) 行为列表与 GIF 资源不匹配。

排查结论：

1. `systray.SetTitle` 在 Windows 上是空实现，且代码从未调用 `systray.SetIcon`，托盘一直显示库默认图标；共享文件夹 gifs/ 中已有 `ameath.ico`（此前计划不入库）但未接线。
2. 用户明确状态使用规则：idle 的 GIF 隔一定时间间隔**随机**播放（取代「资源组织」计划中的顺序合并轮播）；click 不进菜单、点击宠物时播放（现状已满足）；walk 不进菜单、移动时使用（现状已满足）。

## 修改文件

### 1. `src/internal/pet.go` / `action.go` / `gif.go`：每状态多变体

- `Anims` 类型改为 `map[string][]*Animator`（每状态多个变体）。
- `LoadResources` 不再合并同状态 GIF：逐个 GIF 独立加载为变体，全部变体全局归一化画布与脚线（净位移与原「状态内合并+全局归一化」等效）。
- 删除 `gif.go` 的 `loadGIFs`/`mergedLoopCount`（合并逻辑不再需要）。
- `switchAnim` 随机选取变体；音效只在状态真正变化时播放，同状态换变体不重复响。
- `updateAI` idle 分支：`StateTimer%5 == 0`（约 10s）随机换一个 idle 变体。

### 2. `src/internal/action.go`：菜单状态过滤

- `MenuStates` 只收录含 `*.gif` 的状态目录（此前仅按目录存在性判断，无 GIF 的目录也会进菜单）。
- `HasMenuState` 适配变体数组。`internalStates`（walk/click 隐藏）保持不变。

### 3. 托盘图标（新增 `src/internal/icon.go`）

- 启动期（systray 前）生成图标字节存入 `App.TrayIcon`：优先读 `assets/{petName}/icon.ico`、`assets/icon.ico`，否则用 idle 首帧最近邻缩放到 32×32 并编码为 ICO（32bpp BGRA + 全 0 AND 掩码）。
- `OnTrayReady` 调 `systray.SetIcon`，删除 Windows 上无效果的 `SetTitle` 调用。
- `ameath.ico` 可由用户放到 `assets/ameath/icon.ico` 以使用原图。

## 验证方式

1. `cd src && go build -o ../ameath.exe .` 编译通过
2. 托盘显示宠物图标（无 icon.ico 时为自动生成的 32×32 图标）
3. idle 状态下每约 10s 随机切换 idle1~4 变体，变体间无垂直跳动
4. 行为菜单只含 idle；点击宠物播放 click 反应；walk 仅移动时使用
