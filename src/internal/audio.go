package internal

import (
	"os"
	"path/filepath"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

// 初始化音频
func initAudio(*audioOn *bool) error {
	if err := speaker.Init(44100, 44100/10); err != nil {
		*audioOn = false
		return err
	}
	return nil
}

// 播放音频
func playSound(name string) {
	if !audioOn {
		return
	}
	
	file := pet.Sounds[name]
	if file == "" {
		return
	}
	
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