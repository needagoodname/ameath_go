package internal

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	autoStartKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autoStartValue = "Ameath"
)

func exePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

// EnableAutoStart 写入注册表 HKCU\...\Run 实现开机自启
func EnableAutoStart() error {
	exe, err := exePath()
	if err != nil {
		return err
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.SetStringValue(autoStartValue, exe)
}

// DisableAutoStart 删除注册表中的开机自启项
func DisableAutoStart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if err := k.DeleteValue(autoStartValue); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

// IsAutoStartEnabled 查询注册表中是否存在开机自启项
func IsAutoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(autoStartValue)
	return err == nil
}

// SyncAutoStart 根据配置同步注册表状态
func (a *App) SyncAutoStart() {
	if a.Cfg.AutoStart {
		EnableAutoStart()
	} else {
		DisableAutoStart()
	}
}
