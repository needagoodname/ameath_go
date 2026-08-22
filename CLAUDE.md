# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

A Windows desktop pet application ("爱弥斯") — an animated character that walks around the screen, reacts to clicks, and lives in the system tray. Built with Go 1.20 and raw Win32 API (no GUI framework).

## Build & run

Windows 编译（`-H windowsgui` 设置 GUI 子系统，双击启动不弹控制台窗口）：

```bash
cd src
go build -ldflags="-H windowsgui" -o ../ameath.exe .
```

运行（exe 需与 `assets/` 目录同级）：

```bash
../ameath.exe
```

若需在命令行查看 `println` 调试输出，去掉 `-ldflags="-H windowsgui"` 编译成控制台版本即可。

Linux 交叉编译验证（纯 Go 依赖，产物仍需在 Windows 运行）：

```bash
cd src
GOOS=windows CGO_ENABLED=0 go build ./...
```

The module is `github.com/na_me/ameath-go`. Source lives in `src/`, binaries go in the repo root. There is no Makefile or go.sum committed yet — run `go mod tidy` after dependency changes.

Assets (GIFs and MP3/WAV files) are loaded from `assets/` next to the executable (`assetsRoot()` in `action.go`, derived from `os.Executable()`; falls back to `./assets`). Structure: `assets/{petName}/{state}/*.gif` + `*.mp3`/`*.wav`. Each GIF is a separate animation variant (`Anims map[string][]*Animator`); all variants are normalized to a common canvas and feet line (bottom-most opaque row) so nothing jumps. `switchAnim` picks a random variant; in idle, `updateAI` re-picks a random variant every ~10s. Tray icon: `assets/{petName}/icon.ico` or `assets/icon.ico` if present, else generated as a 32×32 ICO from the first idle frame (`icon.go`, stored in `App.TrayIcon` before systray starts).

## Architecture

```
src/
├── main.go              # Entry point: NewApp, init sequence, systray + message loop
└── internal/
    ├── app.go           # App struct (all state), postCmd command channel, playHop, clampToScreen
    ├── pet.go           # Pet struct definition
    ├── action.go        # Animation switching, AI state machine, movement, resource loading, rendering
    ├── gif.go           # Frame/Animator types, GIF decoding with disposal handling
    ├── audio.go         # Audio init (beep/speaker), MP3/WAV playback
    ├── windows.go       # Win32 API bindings, window creation, message loop, wndProc
    ├── icon.go          # Tray icon: icon.ico loading / 32×32 ICO generation from first frame
    ├── ui.go            # Systray menu and event handlers
    ├── autostart.go     # Registry-based auto-start
    └── config.go        # JSON config persistence
```

### Thread ownership model (critical invariant)

All `App` state is owned by the **main thread** (message loop / `wndProc` / timers). Tray goroutines never mutate state directly — they call `app.postCmd(f)` which queues a closure on `app.cmdChan` and posts `WM_APP_EXEC_CMD`; `wndProc` drains the channel and runs closures on the main thread.

```
tray goroutines                  main thread (message loop/wndProc/timers)
  ├─ read ClickedCh           ├─ sole owner of App state
  ├─ systray.MenuItem.*       ├─ WM_TIMER: anim update, movement, AI, render
  └─ postCmd(f) ──chan──▶    ├─ WM_APP_EXEC_CMD: drain cmdChan, run closures
                              └─ WM_DESTROY: kill timers, save config, PostQuitMessage
```

- `postCmd` closures must be self-contained (capture arguments by value).
- `playSound`'s decode goroutine uses only local copies taken on the main thread.
- Quit path: tray quit → `PostMessage(WM_APP_QUIT)` → `DestroyWindow` → `WM_DESTROY` → `PostQuitMessage` → message loop returns → `systray.Quit()`. There is no `quitChan`.
- `CreateWindow()` runs **before** `go systray.Run(...)` so `postCmd` always has a valid hwnd.

### Core data flow

1. `main()` calls `internal.NewApp()`, then `LoadConfig` → `DiscoverPets` → `NewPet` → `InitAudio` → `LoadResources` → `LoadTrayIcon` → `SyncAutoStart` → `CreateWindow` → `go systray.Run(OnTrayReady, OnTrayExit)` → `RunMessageLoop` → `systray.Quit()`.
2. Two timers drive the app: 16ms (~60fps, animation + `updateMovement` + render), 2s (AI state transitions).
3. `wndProc` handles mouse events (drag, click), timer ticks, `WM_APP_EXEC_CMD` (drain command channel), `WM_APP_QUIT`, and `WM_DESTROY`.
4. `OnTrayReady()` (in `ui.go`) sets up a systray menu; every handler posts a closure via `app.postCmd`.
5. `render()` (in `action.go`) draws the current GIF frame to the layered window using `CreateDIBSection` + `UpdateLayeredWindow` with per-pixel alpha.
6. The AI state machine (`updateAI()`, 2s tick) transitions between idle → walk → idle with random sleep and timed reaction states. Walking movement happens per-frame in `updateMovement()` (~2px/frame ≈ 120px/s), with targets clamped by `clampToScreen()` (uses `GetSystemMetrics` — no hardcoded resolution; `SetProcessDPIAware` is called at startup).

### Key types

- **`App`** (`app.go`): aggregates all state — `Pet`, `Cfg`, `AudioOn/Paused` flags, discovered `Pets`, screen size, `cmdChan`. Package-level singleton `app` (required because the `wndProc` callback signature can't carry a receiver).
- **`Pet`** (`pet.go`): holds position, size, state, animation variant map, sound map, window handle, drag state.
- **`Animator`** (`gif.go`): frame array with playback control (current frame, loop count, timing) plus `Width/Height` (canvas size). Thread-safe via `sync.RWMutex`.
- **`Frame`** (`gif.go`): an `*image.RGBA` plus a `time.Duration` delay.

## Known issues

- **No `go.sum` committed**: run `go mod tidy` (on a Windows machine with the Go toolchain) to generate it.
- **Rendering allocates per frame**: `render()` creates a DIB section + compatible DC every frame (60fps) and re-converts pixels in Go. Frames could be pre-converted to BGRA and scaled versions cached.
- **Menu construction reads startup-only state**: `OnTrayReady` reads `Cfg`/`Pet.Name`/`Pets`/`TrayIcon` without synchronization; safe because no writers exist at that point, but don't add post-startup mutations there.
- **DPI is system-aware only** (`SetProcessDPIAware`): on mixed-DPI multi-monitor setups the pet will render wrong (blurry/sized) when moved across monitors; needs per-monitor v2 + `WM_DPICHANGED` to fix.
- **Animation source is GIF**: `image/gif` is 256-color, slow to decode, loads all frames at once; PNG sequence + `meta.json` (explicit `loop`/`play_once`) would decouple ending semantics from GIF header `LoopCount` (0=forever / -1=once / N=N+1 times).

## 经验沉淀（Lessons learned）

排查"缩放变裁剪 / 动作不切换 / 画面冻结"这一系列 bug 沉淀下来的跨项目经验。

### 调试方法论

- **GUI 冻结先分"逻辑卡死"还是"渲染卡死"**：加一个 1Hz 节流的状态打印（`state/frame/playing/renderW/blocking-flags`），看帧索引在不在走、渲染目标尺寸对不对、有没有 gate 拦截渲染。运行时证据优先于反复读代码。
- **自己跑不了的平台，用"埋点 + 用户复现"换观测权**：Linux 跑不了 Win32，给控制台版本加 `println` 让用户在 Windows 复现，一行日志就定位了根因（`away=true` 恒卡死 → render 被跳过）。
- **把纯逻辑抽出来独立模拟**：复制 `Update()`/状态机到独立程序验证帧推进逻辑本身正确，把怀疑范围收敛到环境/状态层。

### Win32 / 分层窗口专项

- **分层窗口的"窗口尺寸"与"分层表面"是两回事**：`SetWindowPos` 改大小 ≠ 表面内容更新；`UpdateLayeredWindow` 的 `psize` 才决定表面。改尺寸/内容的操作必须**同步重渲染**，不能依赖下一帧定时器（可能被 `Paused` 等 gate 跳过）。
- **警惕自锁死循环**：`away=true → 跳过渲染 → 表面陈旧 → WindowFromPoint 拿不到句柄 → away 仍 true → 永不渲染`。任何"读取一个被自己抑制的输出"的门控都可能锁死成永久坏状态，设计时要考虑反馈回路。
- **`WindowFromPoint` 对分层窗口命中测试不可靠**，表面一旧就误判，别用它来 gate 渲染。
- **`SetCapture` 可能丢 `WM_LBUTTONUP`**（Alt-Tab / 捕获被抢 / 窗口外释放）：任何"按下置位/抬起复位"的状态机都要有 `WM_CAPTURECHANGED` 处理 + 看门狗超时兜底。
- **GDI 资源（DC/DIB）"先建新、成功再删旧"原子替换**，失败保留旧 target；先删旧再建新，创建失败后渲染永久停摆。
- **"fail-closed" 守卫是双刃剑**：尺寸一致性守卫防越界，但重建失败时会永久静默渲染；守卫要有恢复路径（每帧重试），别让它 `return` 死路。
- **桌宠应置顶（WS_EX_TOPMOST）**：否则被普通窗口盖住，配合不可靠的遮挡检测就会冻结。

### 工程原则

- **别用启发式猜动画结束**（`CurrentLoop >= 1`）：循环语义（GIF 头 `LoopCount`：0=无限/-1=一次/N=N+1 次）与结束逻辑耦合，是"播完不切换"类坑的温床；改为显式/确定性语义。
- **单点提交 + 中文因果信息**：每个修复单独提交，回滚和 review 省力。
- **抽纯逻辑层**：整个 `internal` 包强依赖 Win32，Linux 编译不过无法测试；把 AI 状态机/缩放/动画推进抽成不依赖 Win32 的包，可测试 + 可跨平台。
