package internal

import (
	"math/rand"
	"os"
	"path/filepath"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// InitAudio 初始化音频（失败则关闭声音）
func (a *App) InitAudio() error {
	if err := speaker.Init(44100, 44100/10); err != nil {
		a.AudioOn = false
		return err
	}
	return nil
}

// playSound 播放状态对应音效（仅主线程调用；文件列表在此取值，
// 解码与播放 goroutine 内只使用局部副本）。
func (a *App) playSound(name string) {
	if !a.AudioOn || a.Paused {
		return
	}

	files := a.Pet.Sounds[name]
	if len(files) == 0 {
		return
	}
	file := files[rand.Intn(len(files))]
	go func() {
		f, err := os.Open(file)
		if err != nil {
			return
		}
		defer f.Close()

		var s beep.StreamSeekCloser
		var format beep.Format

		ext := filepath.Ext(file)
		switch ext {
		case ".mp3":
			s, format, err = mp3.Decode(f)
		case ".wav":
			s, format, err = wav.Decode(f)
		default:
			return
		}

		if err != nil {
			return
		}
		defer s.Close()

		resampled := beep.Resample(4, format.SampleRate, 44100, s)
		speaker.Play(resampled)
	}()
}
