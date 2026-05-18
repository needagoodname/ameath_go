package internal

var (
	pet      *Pet
	audioOn  = true
	quitChan = make(chan bool)
	paused   bool
	away     bool
)
