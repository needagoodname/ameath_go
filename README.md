# 爱弥斯 (Ameath)

一个 Windows 桌面宠物应用 —— 一只会自己在屏幕上散步、被点击会做出反应、常驻系统托盘的动画小角色。纯 Go + 原生 Win32 API 实现，不依赖任何 GUI 框架，只有一个轻量分层窗口（per-pixel alpha 透明）。

## 功能特性

- **自由行动**：AI 状态机驱动，随机在 idle / walk / sleep 之间切换，自己挑选目标点散步（约 120px/s）
- **点击交互**：点击有反应动画和音效，按住可拖动；朝左走自动水平镜像动画
- **常驻托盘**：托盘菜单 + 桌宠右键菜单双入口
  - 行为切换（按动画资源自动生成菜单项，如 happy / eat / sleep）
  - 显示 / 隐藏、暂停 / 继续
  - 置顶开关（开启后每 ~250ms 周期重插 Z 序，浏览器全屏也压得住）
  - 切换宠物（支持多套宠物资源，目录即宠物）
  - 缩放（50% ~ 200%）、音量（0 ~ 100%，0 即静音）
  - 开机自启（写注册表 `HKCU\...\Run`）、退出
- **动画**：GIF 驱动，每状态支持多个变体（如 `idle1~4.gif`）随机轮换；所有变体自动归一化画布与脚线，切换不跳动
- **音效**：每个状态目录下可放任意多个 MP3/WAV，随机播放；音量可调
- **配置持久化**：位置、缩放、音量、置顶、自启、当前宠物等自动保存
- **多架构**：同时产出 amd64 / 386 版本，exe 自带图标与版本信息

## 环境要求

| 项目 | 要求 |
| --- | --- |
| 操作系统 | Windows 10 及以上（由 Go 工具链下限决定；Go 1.21+ 已放弃 Win7/8） |
| 构建工具 | Go 1.26+（见 `src/go.mod`） |
| 架构 | amd64 / 386 |

## 快速开始

直接使用仓库根目录下的 `ameath.exe`（需与 `assets/` 目录同级），或自行构建。

```bash
cd src
go build -ldflags="-H windowsgui" -o ../ameath.exe .
# 运行（exe 与 assets/ 同级）
../ameath.exe
```

- `-H windowsgui` 让 exe 以 GUI 子系统运行，双击启动不弹控制台。
- 去掉该参数编译成控制台版本，即可在命令行看到 `println` 调试输出（`[dbg]` 诊断日志）。

Linux 下做交叉编译验证（产物仍需在 Windows 上运行）：

```bash
cd src
GOOS=windows CGO_ENABLED=0 go build ./...
```

### 重新生成 exe 图标 / 清单 / 版本信息

exe 的图标、清单（`dpi-awareness: system`）和版本信息由 `src/winres/` 下的 go-winres 配置生成，产物是 `src/rsrc_windows_*.syso`，`go build` 自动链接：

```bash
go install github.com/tc-hib/go-winres@latest
cd src
go-winres make   # 读取 winres/winres.json，输出 amd64 / 386 两个 .syso
```

- 图标源文件：`src/winres/ameath.ico`。
- `.syso` 按架构带后缀，只链接匹配当前 `GOARCH` 的那个；换架构需重新 `go-winres make`。
- 改动图标后必须重新生成 `.syso` 再 `go build`。

## 项目结构

```
ameath_go/
├── ameath.exe            # 构建产物
├── assets/               # 宠物资源（运行时加载）
│   └── {petName}/
│       └── {state}/      # 每状态一个目录
│           ├── *.gif     # 动画（多文件 = 多变体）
│           └── *.mp3|wav # 音效（可多个，随机播放）
└── src/
    ├── main.go           # 入口：NewApp → 初始化序列 → 消息循环
    ├── go.mod / go.sum
    ├── winres/           # go-winres 配置（图标 / 清单 / 版本）
    ├── rsrc_windows_*.syso
    └── internal/
        ├── app.go        # App 状态聚合、postCmd 命令通道、playHop
        ├── pet.go        # Pet 结构体与缩放
        ├── action.go     # AI 状态机、移动、动画切换、资源加载、渲染
        ├── gif.go        # GIF 解码 / 动画推进 / 缩放缓存
        ├── audio.go      # beep 音频初始化与播放（MP3/WAV）
        ├── windows.go    # Win32 API 绑定、窗口创建、消息循环、wndProc
        ├── icon.go       # 托盘图标：加载 icon.ico 或从首帧生成 32×32 ICO
        ├── ui.go         # 托盘菜单与事件处理
        ├── autostart.go  # 注册表开机自启
        └── config.go     # JSON 配置读写
```

### 线程所有权模型（关键约定）

所有 `App` 状态只由**主线程**（消息循环 / `wndProc` / 定时器）读写。托盘 goroutine 从不直接改状态，而是通过 `app.postCmd(f)` 把闭包投到 `cmdChan`，再发 `WM_APP_EXEC_CMD` 让 `wndProc` 在主线程排空执行：

```
托盘 goroutine                   主线程（消息循环 / wndProc / 定时器）
  ├─ systray.MenuItem.*        ├─ 唯一持有 App 状态
  └─ postCmd(f) ──chan──▶     ├─ WM_TIMER: 动画 / 移动 / AI / 渲染
                               ├─ WM_APP_EXEC_CMD: 排空 cmdChan 执行闭包
                               └─ WM_DESTROY: 清理定时器、存配置、退出
```

新增任何状态变更都必须走 `postCmd`，不要跨线程改状态。

## 自定义宠物

宠物完全由 `assets/` 目录驱动，无需改代码：

1. 新建目录 `assets/{宠物名}/`，其中每个**状态**一个子目录。
2. 状态目录里放 GIF（一个 GIF 算一个变体，多文件随机轮换）和音频（MP3/WAV，多个随机播）。
3. 启动后托盘/右键菜单的「切换宠物」即可选中。

**状态语义约定**：

| 类别 | 状态名 | 行为 |
| --- | --- | --- |
| 内部状态 | `idle` `walk` `click` | 不进菜单，由 AI / 交互驱动 |
| 一次性行为 | `click` `eat` `happy` | 播完一轮自动切回 idle |
| 菜单行为 | 其它含 GIF 的状态目录（如 `sleep` `eat` `happy`） | 自动出现在行为菜单，可手动触发 |

> 说明：`walk` 走移动用、`click` 被点击/拖拽用、`idle` 待机用，这三个是约定俗成的内部状态；`start` 是启动台词专用（首帧显示后播一句）。

**资源加载规则**（`action.go`）：

- 合法宠物：目录下至少有一个含 `*.gif` 的状态子目录，空目录 / 备份目录会被自动忽略。
- 所有变体统一归一化到最大画布与最大脚线（最底不透明行），避免状态/变体切换时精灵跳动。
- 缩放基于 `BaseWidth/BaseHeight`，按百分比线性缩放。
- 缺省动画：优先 `idle` 第二变体（idle2）→ 否则 `idle` 首个 → 再否则任意首个；完全没有 GIF 视为加载失败。

## 配置

配置保存在 `%AppData%\Ameath\config.json`（`os.UserConfigDir()` 推导，取不到时回退到 exe 目录）。

```json
{
  "auto_start": true,          // 开机自启
  "current_pet": "ameath",     // 当前宠物
  "window_x": 100,             // 上次位置（越界会自动拉回可见区域）
  "window_y": 100,
  "scale_percent": 100,        // 缩放 50~200
  "always_on_top": true,       // 置顶
  "volume_percent": 100        // 音量 0~100（0 = 静音）
}
```

## 已知问题 / 限制

- **DPI 仅系统级感知**（`SetProcessDPIAware` + 清单声明 `system`）：多显示器混合 DPI 时跨屏会拉伸/模糊，需 per-monitor v2 + `WM_DPICHANGED` 才能修复。
- **动画源是 GIF**：256 色、解码慢、一次性全量载入；GIF 的 `LoopCount`（0=无限 / -1=一次 / N=N+1 次）与"播完"判断耦合，是"播完不切换"类坑的温床。后续可考虑 PNG 序列 + `meta.json` 显式声明 `loop` / `play_once`。
- **渲染每帧重建缓存不彻底**：现在复用 memDC/DIB 但每帧仍做一次像素拷贝；可预转 BGRA 并按缩放缓存多档。

## 开发备注

- 资源路径由 `os.Executable()` 推导（`assetsRoot()`），exe 与 `assets/` 必须同级。
- 状态变更、窗口操作必须回主线程（见上文线程模型）。
- 编译期调试：用控制台版本观察 `[dbg]` 日志，能区分"逻辑卡死"（frame 不前进 / Playing=false）与"渲染卡死"（frame 前进但渲染被跳过）。
- 分层窗口注意：`SetWindowPos` 改尺寸 ≠ `UpdateLayeredWindow` 更新表面，改尺寸/内容的操作必须**同步重渲染**，不要依赖下一帧定时器。

## 致谢

- [getlantern/systray](https://github.com/getlantern/systray) — 系统托盘
- [gopxl/beep/v2](https://github.com/gopxl/beep) — 音频播放
- [golang.org/x/sys/windows](https://pkg.go.dev/golang.org/x/sys/windows) — Win32 API 绑定
- [tc-hib/go-winres](https://github.com/tc-hib/go-winres) — exe 图标 / 清单 / 版本信息
