package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config 持久化配置
type Config struct {
	AudioOn      bool   `json:"audio_on"`
	AutoStart    bool   `json:"auto_start"`
	CurrentPet   string `json:"current_pet"`
	WindowX      int32  `json:"window_x"`
	WindowY      int32  `json:"window_y"`
	ScalePercent int    `json:"scale_percent"`
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
		AudioOn:   true,
		AutoStart: false,
		WindowX:   100,
		WindowY:   100,
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
	a.AudioOn = a.Cfg.AudioOn
}

func (a *App) saveConfig() {
	configMu.Lock()
	defer configMu.Unlock()

	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		println("saveConfig: mkdir failed:", err.Error())
		return
	}

	a.Cfg.AudioOn = a.AudioOn
	data, err := json.MarshalIndent(a.Cfg, "", "  ")
	if err != nil {
		println("saveConfig: marshal failed:", err.Error())
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		println("saveConfig: write failed:", err.Error())
	}
}
