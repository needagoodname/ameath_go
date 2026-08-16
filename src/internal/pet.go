package internal

// 宠物
type Pet struct {
	Name        string
	State       string
	X, Y        int32
	Width       int32
	Height      int32
	BaseWidth   int32
	BaseHeight  int32
	TargetX     int32
	TargetY     int32
	Dragging    bool
	DragX       int32
	DragY       int32
	Anims       map[string]*Animator
	CurrentAnim *Animator
	Sounds      map[string][]string
	Hwnd        uintptr
	StateTimer  int
}

func (a *App) NewPet(name string) {
	a.Pet = &Pet{
		Name:    name,
		State:   "idle",
		X:       a.Cfg.WindowX,
		Y:       a.Cfg.WindowY,
		Width:   128,
		Height:  128,
		Anims:   make(map[string]*Animator),
		Sounds:  make(map[string][]string),
	}
	if a.Pet.X == 0 && a.Pet.Y == 0 {
		a.Pet.X = 100
		a.Pet.Y = 100
	}
}

func (p *Pet) SetScale(percent int) {
	p.Width = int32(float64(p.BaseWidth) * float64(percent) / 100.0)
	p.Height = int32(float64(p.BaseHeight) * float64(percent) / 100.0)
}
