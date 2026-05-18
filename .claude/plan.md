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
