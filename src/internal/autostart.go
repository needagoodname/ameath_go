package internal

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
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

	var key windows.Handle
	err = windows.RegCreateKeyEx(
		windows.HKEY_CURRENT_USER,
		windows.StringToUTF16Ptr(`Software\Microsoft\Windows\CurrentVersion\Run`),
		0, nil, windows.REG_OPTION_NON_VOLATILE,
		windows.KEY_SET_VALUE, nil, &key, nil,
	)
	if err != nil {
		return err
	}
	defer windows.RegCloseKey(key)

	val := windows.StringToUTF16(exe)
	return windows.RegSetValueEx(
		key,
		windows.StringToUTF16Ptr("Ameath"),
		0, windows.REG_SZ,
		(*byte)(unsafe.Pointer(&val[0])),
		uint32(len(val)*2),
	)
}

// DisableAutoStart 删除注册表中的开机自启项
func DisableAutoStart() error {
	var key windows.Handle
	err := windows.RegOpenKeyEx(
		windows.HKEY_CURRENT_USER,
		windows.StringToUTF16Ptr(`Software\Microsoft\Windows\CurrentVersion\Run`),
		0, windows.KEY_SET_VALUE, &key,
	)
	if err != nil {
		return err
	}
	defer windows.RegCloseKey(key)

	return windows.RegDeleteValue(key, windows.StringToUTF16Ptr("Ameath"))
}

// IsAutoStartEnabled 查询注册表中是否存在开机自启项
func IsAutoStartEnabled() bool {
	var key windows.Handle
	err := windows.RegOpenKeyEx(
		windows.HKEY_CURRENT_USER,
		windows.StringToUTF16Ptr(`Software\Microsoft\Windows\CurrentVersion\Run`),
		0, windows.KEY_QUERY_VALUE, &key,
	)
	if err != nil {
		return false
	}
	defer windows.RegCloseKey(key)

	_, err = windows.RegQueryValueEx(
		key, windows.StringToUTF16Ptr("Ameath"),
		nil, nil, nil, nil,
	)
	return err == nil
}

// SyncAutoStart 根据配置同步注册表状态
func SyncAutoStart() {
	if Cfg.AutoStart {
		EnableAutoStart()
	} else {
		DisableAutoStart()
	}
}
