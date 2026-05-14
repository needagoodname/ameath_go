package internal

// 宠物
type Pet struct {
	Name        string
	State       string
	X, Y        int32
	Width       int32
	Height      int32
	TargetX     int32
	TargetY     int32
	Dragging    bool
	DragX       int32
	DragY       int32
	Anims       map[string]*Animator
	CurrentAnim *Animator
	Sounds      map[string]string
	Hwnd        uintptr
	StateTimer  int
}