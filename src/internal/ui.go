package internal

import (
	"time"
	
	"github.com/getlantern/systray"
)

// 系统托盘
func onTrayReady() {
	systray.SetTitle("🐱")
	systray.SetTooltip("桌宠")
	
	systray.AddMenuItem("桌宠", "").Disable()
	systray.AddSeparator()
	
	mFeed := systray.AddMenuItem("🍖 喂食", "")
	mPlay := systray.AddMenuItem("🎮 玩耍", "")
	mSleep := systray.AddMenuItem("💤 睡觉", "")
	mWake := systray.AddMenuItem("⏰ 叫醒", "")
	systray.AddSeparator()
	
	mToggle := systray.AddMenuItem("👁️ 显示/隐藏", "")
	mMute := systray.AddMenuItem("🔊 静音", "")
	systray.AddSeparator()
	
	mQuit := systray.AddMenuItem("❌ 退出", "")
	
	go func() {
		for {
			select {
			case <-mFeed.ClickedCh:
				switchAnim("eat")
			case <-mPlay.ClickedCh:
				switchAnim("happy")
				// 跳跃
				for i := 0; i < 3; i++ {
					pet.Y -= 20
					procSetWindowPos.Call(pet.Hwnd, 0, uintptr(pet.X), uintptr(pet.Y), 0, 0, 1|4)
					time.Sleep(100 * time.Millisecond)
					pet.Y += 20
					procSetWindowPos.Call(pet.Hwnd, 0, uintptr(pet.X), uintptr(pet.Y), 0, 0, 1|4)
					time.Sleep(100 * time.Millisecond)
				}
			case <-mSleep.ClickedCh:
				switchAnim("sleep")
			case <-mWake.ClickedCh:
				switchAnim("idle")
			case <-mToggle.ClickedCh:
				style, _, _ := procGetWindowLong.Call(pet.Hwnd, GWL_STYLE)
				if style&WS_VISIBLE != 0 {
					procShowWindow.Call(pet.Hwnd, 0)
				} else {
					procShowWindow.Call(pet.Hwnd, SW_SHOWNOACTIVATE)
				}
			case <-mMute.ClickedCh:
				audioOn = !audioOn
				if audioOn {
					mMute.SetTitle("🔊 静音")
				} else {
					mMute.SetTitle("🔇 取消静音")
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				close(quitChan)
				procPostQuitMessage.Call(0)
			}
		}
	}()
}

func onTrayExit() {}