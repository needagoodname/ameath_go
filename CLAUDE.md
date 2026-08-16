# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

A Windows desktop pet application ("爱弥斯") — an animated character that walks around the screen, reacts to clicks, and lives in the system tray. Built with Go 1.20 and raw Win32 API (no GUI framework).

## Build & run

```bash
cd src
go build -o ../ameath.exe .
```

The module is `github.com/na_me/ameath-go`. Source lives in `src/`, binaries go in the repo root. There is no Makefile or go.sum committed yet — run `go mod tidy` after dependency changes.

This is a Windows-only application (`golang.org/x/sys/windows`, `CreateWindowExW`, `UpdateLayeredWindow`). Cross-compilation is not possible.

Assets (GIFs and MP3/WAV files) are loaded from `assets/` next to the executable (`assetsRoot()` in `action.go`, derived from `os.Executable()`; falls back to `./assets`). Structure: `assets/{petName}/{state}/*.gif` + `*.mp3`/`*.wav`. All GIFs in a state directory are merged into one animation: frames concatenated in filename order, canvases normalized to the largest size, each GIF aligned by its feet line (bottom-most opaque row) so variants don't jump.

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
    ├── windows.go       # Win32 API bindings, window creation, message loop, wndProc, occlusion
    ├── ui.go            # Systray menu and event handlers
    ├── autostart.go     # Registry-based auto-start
    └── config.go        # JSON config persistence
```

### Thread ownership model (critical invariant)

All `App` state is owned by the **main thread** (message loop / `wndProc` / timers). Tray goroutines never mutate state directly — they call `app.postCmd(f)` which queues a closure on `app.cmdChan` and posts `WM_APP_EXEC_CMD`; `wndProc` drains the channel and runs closures on the main thread.

```
tray goroutines                  main thread (message loop/wndProc/timers)
  ├─ read ClickedCh           ├─ sole owner of App state
  ├─ systray.MenuItem.*       ├─ WM_TIMER: anim update, movement, AI, render, occlusion
  └─ postCmd(f) ──chan──▶    ├─ WM_APP_EXEC_CMD: drain cmdChan, run closures
                              └─ WM_DESTROY: kill timers, save config, PostQuitMessage
```

- `postCmd` closures must be self-contained (capture arguments by value).
- `playSound`'s decode goroutine uses only local copies taken on the main thread.
- Quit path: tray quit → `PostMessage(WM_APP_QUIT)` → `DestroyWindow` → `WM_DESTROY` → `PostQuitMessage` → message loop returns → `systray.Quit()`. There is no `quitChan`.
- `CreateWindow()` runs **before** `go systray.Run(...)` so `postCmd` always has a valid hwnd.

### Core data flow

1. `main()` calls `internal.NewApp()`, then `LoadConfig` → `DiscoverPets` → `NewPet` → `InitAudio` → `LoadResources` → `SyncAutoStart` → `CreateWindow` → `go systray.Run(OnTrayReady, OnTrayExit)` → `RunMessageLoop` → `systray.Quit()`.
2. Three timers drive the app: 16ms (~60fps, animation + `updateMovement` + render), 2s (AI state transitions), 5s (occlusion detection).
3. `wndProc` handles mouse events (drag, click), timer ticks, `WM_APP_EXEC_CMD` (drain command channel), `WM_APP_QUIT`, and `WM_DESTROY`.
4. `OnTrayReady()` (in `ui.go`) sets up a systray menu; every handler posts a closure via `app.postCmd`.
5. `render()` (in `action.go`) draws the current GIF frame to the layered window using `CreateDIBSection` + `UpdateLayeredWindow` with per-pixel alpha.
6. The AI state machine (`updateAI()`, 2s tick) transitions between idle → walk → idle with random sleep and timed reaction states. Walking movement happens per-frame in `updateMovement()` (~2px/frame ≈ 120px/s), with targets clamped by `clampToScreen()` (uses `GetSystemMetrics` — no hardcoded resolution; `SetProcessDPIAware` is called at startup).

### Key types

- **`App`** (`app.go`): aggregates all state — `Pet`, `Cfg`, `AudioOn/Paused/Away` flags, discovered `Pets`, screen size, `cmdChan`. Package-level singleton `app` (required because the `wndProc` callback signature can't carry a receiver).
- **`Pet`** (`pet.go`): holds position, size, state, animation map, sound map, window handle, drag state.
- **`Animator`** (`gif.go`): frame array with playback control (current frame, loop count, timing) plus `Width/Height` (canvas size). Thread-safe via `sync.RWMutex`.
- **`Frame`** (`gif.go`): an `*image.RGBA` plus a `time.Duration` delay.

## Known issues

- **No `go.sum` committed**: run `go mod tidy` (on a Windows machine with the Go toolchain) to generate it.
- **Rendering allocates per frame**: `render()` creates a DIB section + compatible DC every frame (60fps) and re-converts pixels in Go. Frames could be pre-converted to BGRA and scaled versions cached.
- **Occlusion heuristic**: `checkOcclusion()` samples 5 points near the sprite center and requires alpha ≥ 32; a pet whose center pixels are transparent could still be misdetected as occluded.
- **Menu construction reads startup-only state**: `OnTrayReady` reads `Cfg`/`Pet.Name`/`Pets` without synchronization; safe because no writers exist at that point, but don't add post-startup mutations there.
