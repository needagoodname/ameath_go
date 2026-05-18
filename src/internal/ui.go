package internal

import (
	"fmt"
	"time"

	"github.com/getlantern/systray"
)

// 系统托盘
func OnTrayReady() {
	systray.SetTitle("🐱")
	systray.SetTooltip("桌宠")

	systray.AddMenuItem("桌宠", "").Disable()
	systray.AddSeparator()

	// 行为菜单（动态生成）
	stateMenuItems := make(map[string]*systray.MenuItem)
	for _, state := range MenuStates() {
		item := systray.AddMenuItem(state, "")
		stateMenuItems[state] = item
	}
	systray.AddSeparator()

	mToggle := systray.AddMenuItem("显示/隐藏", "")
	mMute := systray.AddMenuItem("静音", "")
	mPause := systray.AddMenuItem("暂停", "")
	systray.AddSeparator()

	// 宠物切换子菜单
	mSwitchPet := systray.AddMenuItem("切换宠物", "")
	pets := AvailablePets()
	if len(pets) <= 1 {
		mSwitchPet.Disable()
	}
	petMenuItems := make(map[string]*systray.MenuItem)
	for _, name := range pets {
		title := name
		if pet != nil && name == pet.Name {
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
		if Cfg.ScalePercent == pct {
			title = "✓ " + title
		}
		item := mScale.AddSubMenuItem(title, "")
		scaleMenuItems[pct] = item
	}

	systray.AddSeparator()

	mAutoStart := systray.AddMenuItem("开机启动", "")
	if Cfg.AutoStart {
		mAutoStart.SetTitle("开机启动 ✓")
	}
	systray.AddSeparator()

	mQuit := systray.AddMenuItem("退出", "")

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				style, _, _ := procGetWindowLong.Call(pet.Hwnd, GWL_STYLE)
				if style&WS_VISIBLE != 0 {
					procShowWindow.Call(pet.Hwnd, 0)
				} else {
					procShowWindow.Call(pet.Hwnd, SW_SHOWNOACTIVATE)
				}
			case <-mMute.ClickedCh:
				audioOn = !audioOn
				Cfg.AudioOn = audioOn
				saveConfig()
				if audioOn {
					mMute.SetTitle("静音")
				} else {
					mMute.SetTitle("取消静音")
				}
			case <-mPause.ClickedCh:
				paused = !paused
				if paused {
					mPause.SetTitle("继续")
				} else {
					mPause.SetTitle("暂停")
					away = false
				}
			case <-mAutoStart.ClickedCh:
				Cfg.AutoStart = !Cfg.AutoStart
				if Cfg.AutoStart {
					mAutoStart.SetTitle("开机启动 ✓")
					EnableAutoStart()
				} else {
					mAutoStart.SetTitle("开机启动")
					DisableAutoStart()
				}
				saveConfig()
			case <-mQuit.ClickedCh:
				systray.Quit()
				close(quitChan)
				procPostQuitMessage.Call(0)
			}
		}
	}()

	// 行为菜单事件（动态）
	for state, item := range stateMenuItems {
		go func(s string, mi *systray.MenuItem) {
			for range mi.ClickedCh {
				switchAnim(s)
				if s == "happy" {
					for i := 0; i < 3; i++ {
						pet.Y -= 20
						procSetWindowPos.Call(pet.Hwnd, 0, uintptr(pet.X), uintptr(pet.Y), 0, 0, 1|4)
						time.Sleep(100 * time.Millisecond)
						pet.Y += 20
						procSetWindowPos.Call(pet.Hwnd, 0, uintptr(pet.X), uintptr(pet.Y), 0, 0, 1|4)
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
		}(state, item)
	}

	// 宠物切换子菜单事件
	for name, item := range petMenuItems {
		go func(n string, mi *systray.MenuItem) {
			for range mi.ClickedCh {
				if n != pet.Name {
					SwitchPet(n)
					for pn, pmi := range petMenuItems {
						if pn == n {
							pmi.SetTitle("✓ " + pn)
						} else {
							pmi.SetTitle(pn)
						}
					}
					// 更新行为菜单项可见性
					for s, smi := range stateMenuItems {
						if HasMenuState(s) {
							smi.Show()
						} else {
							smi.Hide()
						}
					}
				}
			}
		}(name, item)
	}

	// 放缩子菜单事件
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
}

func OnTrayExit() {}
