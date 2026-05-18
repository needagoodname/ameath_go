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

Assets (GIFs and MP3/WAV files) are loaded from `./assets/` relative to the working directory at runtime.

## Architecture

```
src/
├── main.go              # Entry point: creates Pet, inits audio, launches systray + window
└── internal/
    ├── pet.go           # Pet struct definition
    ├── action.go        # Animation switching, AI state machine, resource loading, rendering
    ├── gif.go           # Frame/Animator types, GIF decoding with disposal handling
    ├── audio.go         # Audio init (beep/speaker), MP3/WAV playback
    ├── windows.go       # Win32 API bindings, window creation, message loop, wndProc
    ├── ui.go            # Systray menu and event handlers
    ├── typeError.go     # Custom error type (has a syntax bug — see Known issues)
    └── typeLog.go       # Zerolog-based file+console logger (declares `package logger`)
```

### Core data flow

1. `main()` creates a `Pet` struct, initializes audio, then calls `internal.loadResources(&pet)` to load GIF animations and audio files from `./assets/` (e.g. `idle.gif`, `walk.gif`, `eat.mp3`).
2. `internal.createWindow()` (in `windows.go`) registers a window class, creates a transparent layered window, and enters the Win32 message loop. Two timers drive the app: a 16ms timer (~60fps) for animation+render, and a 2s timer for AI state updates.
3. `wndProc` handles mouse events (drag, click), timer ticks (render + AI), and window destruction.
4. `internal.onTrayReady()` (in `ui.go`) sets up a systray menu with feed/play/sleep/wake/toggle/mute/quit items that trigger animation switches via `switchAnim()`.
5. `render()` (in `action.go`) draws the current GIF frame to the layered window using `CreateDIBSection` + `UpdateLayeredWindow` with per-pixel alpha.
6. The AI state machine (`updateAI()`) transitions between idle → walk → idle, with random sleep and timed reaction states (click/eat/happy → back to idle).

### Key types

- **`Pet`** (`pet.go`): holds position, size, state, animation map, sound map, window handle, drag state.
- **`Animator`** (`gif.go`): frame array with playback control (current frame, loop count, timing). Thread-safe via `sync.RWMutex`.
- **`Frame`** (`gif.go`): an `*image.RGBA` plus a `time.Duration` delay.

## Known issues

- **`src/internal/typeLog.go`** declares `package logger` — all other files in the directory use `package internal`. This causes a compile error.
- **`src/internal/typeError.go`** has a syntax error: `func (msg string, err error) *typeError {` is missing the function name (should be something like `func newTypeError`).
- **Unexported cross-package calls**: `main.go` calls `internal.initAudio`, `internal.loadResources`, `internal.onTrayReady`, `internal.onTrayExit` — these are lowercase (unexported) and won't compile from outside the `internal` package.
- **Missing package-level variables**: `action.go`, `audio.go`, `windows.go`, `ui.go` reference `pet`, `audioOn`, `quitChan` as if they were package-level variables in `internal`, but they're declared as locals in `main()` (in `main` package).
- **No `go.sum`**: run `go mod tidy` after fixing the above to generate it.
- **Hardcoded screen bounds**: AI clamps to 1920×1080 in `action.go:55-63`.
