package internal

import (
	"fmt"

	"github.com/getlantern/systray"
)

// 系统托盘
func OnTrayReady() {
	println("tray ready")
	if len(app.TrayIcon) > 0 {
		systray.SetIcon(app.TrayIcon)
	}
	systray.SetTooltip("桌宠")

	systray.AddMenuItem("桌宠", "").Disable()
	systray.AddSeparator()

	// 行为菜单（动态生成）
	stateMenuItems := make(map[string]*systray.MenuItem)
	for _, state := range app.MenuStates() {
		item := systray.AddMenuItem(state, "")
		stateMenuItems[state] = item
	}
	systray.AddSeparator()

	mToggle := systray.AddMenuItem("显示/隐藏", "")
	mMute := systray.AddMenuItem("静音", "")
	if !app.AudioOn {
		mMute.SetTitle("取消静音")
	}
	mPause := systray.AddMenuItem("暂停", "")
	mTopmost := systray.AddMenuItem("置顶", "")
	if app.Cfg.AlwaysOnTop {
		mTopmost.SetTitle("置顶 ✓")
	}
	systray.AddSeparator()

	// 宠物切换子菜单
	mSwitchPet := systray.AddMenuItem("切换宠物", "")
	pets := app.Pets
	if len(pets) <= 1 {
		mSwitchPet.Disable()
	}
	petMenuItems := make(map[string]*systray.MenuItem)
	for _, name := range pets {
		title := name
		if app.Pet != nil && name == app.Pet.Name {
			title = "✓ " + name
		}
		item := mSwitchPet.AddSubMenuItem(title, "")
		petMenuItems[name] = item
	}

	// 放缩子菜单
	mScale := systray.AddMenuItem("缩放", "")
	scalePresets := []int{50, 75, 100, 125, 150, 200}
	scaleMenuItems := make(map[int]*systray.MenuItem)
	for _, pct := range scalePresets {
		title := fmt.Sprintf("%d%%", pct)
		if app.Cfg.ScalePercent == pct {
			title = "✓ " + title
		}
		item := mScale.AddSubMenuItem(title, "")
		scaleMenuItems[pct] = item
	}

	// 音量子菜单（0~100%）
	mVolume := systray.AddMenuItem("音量", "")
	volumePresets := []int{0, 25, 50, 75, 100}
	volumeMenuItems := make(map[int]*systray.MenuItem)
	for _, pct := range volumePresets {
		title := fmt.Sprintf("%d%%", pct)
		if app.Cfg.VolumePercent == pct {
			title = "✓ " + title
		}
		item := mVolume.AddSubMenuItem(title, "")
		volumeMenuItems[pct] = item
	}

	systray.AddSeparator()

	mAutoStart := systray.AddMenuItem("开机启动", "")
	if app.Cfg.AutoStart {
		mAutoStart.SetTitle("开机启动 ✓")
	}
	systray.AddSeparator()

	mQuit := systray.AddMenuItem("退出", "")

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				app.postCmd(func() {
					gwl := int32(GWL_STYLE)
					style, _, _ := procGetWindowLong.Call(app.Pet.Hwnd, uintptr(gwl))
					if style&WS_VISIBLE != 0 {
						procShowWindow.Call(app.Pet.Hwnd, 0)
					} else {
						procShowWindow.Call(app.Pet.Hwnd, SW_SHOWNOACTIVATE)
					}
				})
			case <-mMute.ClickedCh:
				app.postCmd(func() {
					app.AudioOn = !app.AudioOn
					app.Cfg.AudioOn = app.AudioOn
					app.saveConfig()
					if app.AudioOn {
						mMute.SetTitle("静音")
					} else {
						mMute.SetTitle("取消静音")
					}
				})
			case <-mPause.ClickedCh:
				app.postCmd(func() {
					app.Paused = !app.Paused
					if app.Paused {
						mPause.SetTitle("继续")
					} else {
						mPause.SetTitle("暂停")
					}
				})
			case <-mTopmost.ClickedCh:
				app.postCmd(func() {
					app.setTopmost(!app.isTopmost())
					app.saveConfig()
					if app.Cfg.AlwaysOnTop {
						mTopmost.SetTitle("置顶 ✓")
					} else {
						mTopmost.SetTitle("置顶")
					}
				})
			case <-mAutoStart.ClickedCh:
				app.postCmd(func() {
					app.Cfg.AutoStart = !app.Cfg.AutoStart
					if app.Cfg.AutoStart {
						mAutoStart.SetTitle("开机启动 ✓")
						EnableAutoStart()
					} else {
						mAutoStart.SetTitle("开机启动")
						DisableAutoStart()
					}
					app.saveConfig()
				})
			case <-mQuit.ClickedCh:
				procPostMessage.Call(app.Pet.Hwnd, WM_APP_QUIT, 0, 0)
			}
		}
	}()

	// 行为菜单事件（动态）
	for state, item := range stateMenuItems {
		go func(s string, mi *systray.MenuItem) {
			for range mi.ClickedCh {
				app.postCmd(func() {
					app.switchAnim(s)
					if s == "happy" {
						app.playHop()
					}
				})
			}
		}(state, item)
	}

	// 宠物切换子菜单事件
	for name, item := range petMenuItems {
		go func(n string, mi *systray.MenuItem) {
			for range mi.ClickedCh {
				app.postCmd(func() {
					if n == app.Pet.Name {
						return
					}
					if err := app.SwitchPet(n); err != nil {
						println("switch pet failed:", err.Error())
						return
					}
					for pn, pmi := range petMenuItems {
						if pn == n {
							pmi.SetTitle("✓ " + pn)
						} else {
							pmi.SetTitle(pn)
						}
					}
					// 更新行为菜单项可见性
					for s, smi := range stateMenuItems {
						if app.HasMenuState(s) {
							smi.Show()
						} else {
							smi.Hide()
						}
					}
				})
			}
		}(name, item)
	}

	// 放缩子菜单事件
	for pct, item := range scaleMenuItems {
		go func(percent int, mi *systray.MenuItem) {
			for range mi.ClickedCh {
				app.postCmd(func() {
					if percent == app.Cfg.ScalePercent {
						return
					}
					app.resizeWindow(percent)
					for sp, smi := range scaleMenuItems {
						if sp == percent {
							smi.SetTitle(fmt.Sprintf("✓ %d%%", sp))
						} else {
							smi.SetTitle(fmt.Sprintf("%d%%", sp))
						}
					}
				})
			}
		}(pct, item)
	}

	// 音量子菜单事件
	for pct, item := range volumeMenuItems {
		go func(percent int, mi *systray.MenuItem) {
			for range mi.ClickedCh {
				app.postCmd(func() {
					if percent == app.Cfg.VolumePercent {
						return
					}
					app.Cfg.VolumePercent = percent
					app.saveConfig()
					for vp, vmi := range volumeMenuItems {
						if vp == percent {
							vmi.SetTitle(fmt.Sprintf("✓ %d%%", vp))
						} else {
							vmi.SetTitle(fmt.Sprintf("%d%%", vp))
						}
					}
				})
			}
		}(pct, item)
	}
}

func OnTrayExit() {}
