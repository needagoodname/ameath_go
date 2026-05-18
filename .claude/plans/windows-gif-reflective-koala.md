# Windows 桌宠 — 开机启动、配置持久化、多宠资源隔离

## Context

当前项目存在 9 个编译阻塞问题（包级变量缺失、函数不可见、语法错误、缺失常量等），必须先修复。然后添加三个功能：开机自启、配置持久化、多宠物资源隔离。

## 实现步骤

### Phase 1: 清理死代码

1. **删除 `src/internal/typeError.go`** — 有语法错误且未被任何地方引用
2. **删除 `src/internal/typeLog.go`** — 包名错误（`logger` vs `internal`），且 zerolog 不在 go.mod 中，未被引用

### Phase 2: 修复编译

3. **新建 `src/internal/state.go`** — 声明包级变量 `pet *Pet`、`audioOn bool`、`quitChan chan bool`（目前只在 main.go 的 main() 中作为局部变量存在，但所有 internal 文件都直接引用它们）
4. **`action.go`** — 添加 `"math/rand"` 导入
5. **`windows.go`** — 添加 `WM_PAINT = 0x000F` 常量
6. **`audio.go`** — 修复 `func initAudio(*audioOn *bool)` 语法错误为 `func InitAudio() error`
7. **导出函数** — 将所有被 main.go 调用的函数改为大写导出：
   - `ui.go`: `onTrayReady` → `OnTrayReady`、`onTrayExit` → `OnTrayExit`
   - `audio.go`: `initAudio` → `InitAudio`
   - `action.go`: `loadResources` → `LoadResources`
   - `windows.go`: `createWindow` → `RunMessageLoop`
8. **`main.go` 重写** — 移除 pet/audioOn/quitChan 局部变量声明，调用导出的 internal 函数

### Phase 3: 配置持久化

9. **新建 `src/internal/config.go`**：
   - `Config` 结构体：`AudioOn`、`AutoStart`、`CurrentPet`、`WindowX`、`WindowY`
   - 配置文件路径：`%APPDATA%\Ameath\config.json`（通过 `os.UserConfigDir()` 获取）
   - `LoadConfig()` — 读取 JSON，不存在时创建默认值
   - `saveConfig()` — 写 JSON，创建父目录，加 `sync.Mutex` 防并发
   - Setter 函数自动触发保存

### Phase 4: 开机自启

10. **新建 `src/internal/autostart.go`**：
    - `EnableAutoStart()` — 写入 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` 的 `"Ameath"` 值（REG_SZ，exe 路径）
    - `DisableAutoStart()` — 删除该注册表值
    - `IsAutoStartEnabled()` — 查询该值是否存在
    - 使用 `golang.org/x/sys/windows`（已在 go.mod 中）
    - `SyncAutoStart()` — 根据当前 config 同步注册表状态

11. **`ui.go`** — 添加 `🚀 开机启动` 菜单项，带 ✓ 状态标识，点击时切换并调用 Enable/Disable

### Phase 5: 多宠物资源隔离

资产目录采用**状态子目录**组织：每个宠物是一个目录，每个状态是该目录下的一个子目录，状态目录内含一个 GIF 和若干音频文件。状态名由子目录名动态发现，不硬编码。

12. **新建 `src/internal/pets.go`**：
    - `DiscoverPets()` — 扫描 `./assets/` 子目录，返回宠物名列表
    - `AvailablePets()` — 返回缓存的列表
    - `SwitchPet(name)` — 清空当前动画/音频，重新加载新宠物资源，更新 config

13. **`action.go`** — 重写 `LoadResources(petName)`：
    - 扫描 `./assets/{petName}/` 下所有子目录，每个子目录名即为一个状态
    - 每个状态目录下：找唯一的 `.gif` 作为动画，收集所有 `.mp3` `.wav` 作为音频池
    - 删除硬编码的状态列表和 `assetsDir` 全局变量
    - 删除 `loadResources` → 导出为 `LoadResources`

14. **`pet.go`** — 两处修改：
    - `Sounds` 字段类型 `map[string]string` → `map[string][]string`（支持多音频）
    - 添加 `NewPet(name string)` 构造函数，从 config 读取初始位置

15. **`audio.go`** — `playSound` 从取单个文件改为从音频池随机选取：
    ```go
    files := pet.Sounds[name]
    if len(files) > 0 {
        file := files[rand.Intn(len(files))]
        // 播放 file...
    }
    ```

16. **`ui.go`** — 添加 `🔄 切换宠物` 子菜单，列出所有发现的宠物，当前宠物前加 ✓

### Phase 6: 收尾

17. **`main.go`** — 串联所有步骤：
    ```
    LoadConfig → DiscoverPets → NewPet → InitAudio → LoadResources → SyncAutoStart → systray + RunMessageLoop
    ```
18. 运行 `go mod tidy` 生成 `go.sum`

## 资产目录新结构

```
assets/
  cat/                    ← 一个宠物 = 一个子目录
    idle/                 ← 一个状态 = 一个子目录（状态名 = 目录名）
      idle.gif            ← 该状态的 GIF（目录下唯一的 .gif）
      purr.mp3            ← 若干音频文件，任意命名
      meow.wav
    walk/
      walk.gif
      step1.wav
      step2.wav
    sleep/
      sleep.gif
      snore.mp3
    eat/
      eat.gif
      crunch.mp3
    happy/
      happy.gif
      chirp.mp3
      trill.wav
    click/
      click.gif
      squeak.mp3
  dog/
    idle/
      idle.gif
      pant.mp3
      bark.wav
    ...
```

发现规则：
- 扫描 `assets/{petName}/` 下所有子目录，子目录名即为状态名（不硬编码状态种类）
- 每个状态目录下找唯一的 `.gif` 作为动画帧源
- 每个状态目录下所有 `.mp3`、`.wav` 文件作为该状态的音频池，播放时随机选取

## 关键设计决策

- 配置变更立即持久化（不等到退出），窗口位置在拖拽释放时保存
- 注册表操作失败不回滚 UI 状态，只打印错误
- 资产缺失不崩溃：`LoadResources` 已有 graceful degradation
- 使用 `windows.RegCreateKeyEx` 等标准 API 操作注册表，无需新增依赖

## 验证方法

```bash
cd src
go build -o ../ameath.exe .
go vet ./...
```
- 启动程序：检查 `%APPDATA%\Ameath\config.json` 是否自动创建
- 切换开机启动：检查注册表 `HKCU\...\Run\Ameath` 是否写入/删除
- 切换宠物：确认动画和音频从对应子目录加载
- 修改配置重启：确认状态持久化

---

## Phase 7: 暂停 & 后台自动静止

### 功能说明

**暂停**：用户通过托盘菜单手动暂停/恢复。暂停时动画、AI、渲染、音频全部停止，保留最后一帧画面。重启后不保持暂停状态。

**后台自动静止**：检测宠物窗口是否被其他窗口完全遮挡，若遮挡则自动停止动画渲染和 AI 节约资源，遮挡解除后自动恢复。

### 状态模型

```
                        ┌──────────┐
              拖拽结束   │  NORMAL  │  遮挡检测触发
             ┌─────────│ (运行中)  │──────────┐
             │         └─────┬────┘          │
             ▼               │ 用户暂停       ▼
       ┌──────────┐          ▼          ┌──────────┐
       │ DRAGGING │     ┌──────────┐    │   AWAY   │  遮挡解除
       │ (拖拽中)  │     │  PAUSED  │    │ (后台静止) │──────────┐
       └──────────┘     └──────────┘    └──────────┘          │
                             ▲              │  用户暂停         │
                             │              ▼                   │
                             │         ┌────────────┐          │
                             └─────────│ PAUSED+AWAY │          │
                                       └────────────┘          │
                                           ▲                   │
                                           │  遮挡解除          │
                                           └───────────────────┘
```

- NORMAL → PAUSED：用户点击暂停
- PAUSED → NORMAL：用户点击恢复（同时清除 away）
- NORMAL → AWAY：遮挡检测判定完全被遮挡时切入 idle 动画
- AWAY → NORMAL：遮挡检测判定恢复可见
- AWAY → PAUSED+AWAY：用户在静止时暂停
- PAUSED+AWAY → PAUSED：遮挡解除但用户仍暂停
- 暂停时不触发遮挡检测

### 步骤

19. **`state.go`** — 新增两个状态变量：
    ```go
    paused bool   // 用户主动暂停
    away   bool   // 后台自动静止
    ```

20. **`windows.go`** — 三处改动：

    **a)** 新增 `WindowFromPoint` API 绑定
    **b)** 新增遮挡检测函数：
    ```go
    func checkOcclusion() bool {
        if pet.Hwnd == 0 { return false }
        // 采样宠物矩形中心 + 四角
        pts := []POINT{
            {pet.X + pet.Width/2, pet.Y + pet.Height/2},
            {pet.X + 2, pet.Y + 2},
            {pet.X + pet.Width - 2, pet.Y + 2},
            {pet.X + 2, pet.Y + pet.Height - 2},
            {pet.X + pet.Width - 2, pet.Y + pet.Height - 2},
        }
        for _, pt := range pts {
            h, _, _ := procWindowFromPoint.Call(uintptr(pt.X) | uintptr(pt.Y)<<32)
            if h == pet.Hwnd { return false }
        }
        return true // 完全遮挡
    }
    ```
    **c)** `wndProc` 定时器门控：
    - 帧定时器 (ID 1)：`if paused || away { return 0 }`
    - AI 定时器 (ID 2)：`if paused || away { return 0 }`
    - 新增遮挡检测定时器 (ID 3, 5s)：`if paused || pet.Dragging { return 0 }` 否则调 `checkOcclusion()`
    **d)** `RunMessageLoop` 创建窗口后注册定时器 3：`procSetTimer.Call(hwnd, 3, 5000, 0)`

21. **`ui.go`** — 添加暂停菜单项（放在静音和切换宠物之间）：
    ```go
    mPause := systray.AddMenuItem("⏸️ 暂停", "")
    ```
    点击切换 `paused`，动态改标题 `"⏸️ 暂停"` ↔ `"▶️ 继续"`，恢复时清零 `away`

22. **`action.go`** — `switchAnim` 开头增加 `if paused { return }`，暂停时托盘交互项（喂食/玩耍）无效果

23. **`audio.go`** — `playSound` 增加 `if paused { return }` 守卫

### 设计要点

- 暂停和静止互不冲突：暂停优先级更高（用户意图），静止是自动的
- 拖拽时不做遮挡检测（避免用户拖动时误判）
- 静止时切入 idle 动画让画面静止在自然状态
- 不持久化暂停/静止状态
