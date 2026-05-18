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

func NewPet(name string) {
	pet = &Pet{
		Name:    name,
		State:   "idle",
		X:       Cfg.WindowX,
		Y:       Cfg.WindowY,
		Width:   128,
		Height:  128,
		Anims:   make(map[string]*Animator),
		Sounds:  make(map[string][]string),
	}
	if pet.X == 0 && pet.Y == 0 {
		pet.X = 100
		pet.Y = 100
	}
}

func (p *Pet) SetScale(percent int) {
	p.Width = int32(float64(p.BaseWidth) * float64(percent) / 100.0)
	p.Height = int32(float64(p.BaseHeight) * float64(percent) / 100.0)
}