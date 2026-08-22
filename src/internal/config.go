package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config 持久化配置
type Config struct {
	AutoStart    bool   `json:"auto_start"`
	CurrentPet   string `json:"current_pet"`
	WindowX      int32  `json:"window_x"`
	WindowY      int32  `json:"window_y"`
	ScalePercent int    `json:"scale_percent"`
	AlwaysOnTop  bool   `json:"always_on_top"`
	// VolumePercent 音量百分比：0~100，100=原音量，0=静音。
	VolumePercent int `json:"volume_percent"`
}

var configMu sync.Mutex

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "config.json")
	}
	return filepath.Join(dir, "Ameath", "config.json")
}

// LoadConfig 加载配置，不存在则创建默认值
func (a *App) LoadConfig() {
	a.Cfg = Config{
		AutoStart:     false,
		WindowX:       100,
		WindowY:       100,
		AlwaysOnTop:   true,
		VolumePercent: 100,
	}

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		a.saveConfig()
		return
	}
	json.Unmarshal(data, &a.Cfg)
	if a.Cfg.ScalePercent <= 0 {
		a.Cfg.ScalePercent = 100
	}
	// 音量限制在 0~100：旧配置缺省（预置 100 保留）或超出范围的
	// 历史值（此前最高 300%）统一钳制到合法区间；0 为静音。
	if a.Cfg.VolumePercent < 0 {
		a.Cfg.VolumePercent = 0
	} else if a.Cfg.VolumePercent > 100 {
		a.Cfg.VolumePercent = 100
	}
}

func (a *App) saveConfig() {
	configMu.Lock()
	defer configMu.Unlock()

	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		println("saveConfig: mkdir failed:", err.Error())
		return
	}

	data, err := json.MarshalIndent(a.Cfg, "", "  ")
	if err != nil {
		println("saveConfig: marshal failed:", err.Error())
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		println("saveConfig: write failed:", err.Error())
	}
}
