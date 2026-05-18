package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config 持久化配置
type Config struct {
	AudioOn    bool   `json:"audio_on"`
	AutoStart  bool   `json:"auto_start"`
	CurrentPet string `json:"current_pet"`
	WindowX    int32  `json:"window_x"`
	WindowY      int32  `json:"window_y"`
	ScalePercent int    `json:"scale_percent"`
}

// Cfg 全局配置实例
var Cfg Config

var configMu sync.Mutex

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "config.json")
	}
	return filepath.Join(dir, "Ameath", "config.json")
}

// LoadConfig 加载配置，不存在则创建默认值
func LoadConfig() {
	Cfg = Config{
		AudioOn:   true,
		AutoStart: false,
		WindowX:   100,
		WindowY:   100,
	}

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		saveConfig()
		return
	}
	json.Unmarshal(data, &Cfg)
	if Cfg.ScalePercent <= 0 {
		Cfg.ScalePercent = 100
	}
	audioOn = Cfg.AudioOn
}

func saveConfig() {
	configMu.Lock()
	defer configMu.Unlock()

	path := configPath()
	os.MkdirAll(filepath.Dir(path), 0755)

	Cfg.AudioOn = audioOn
	data, _ := json.MarshalIndent(Cfg, "", "  ")
	os.WriteFile(path, data, 0644)
}
