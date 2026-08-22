package internal

import "time"

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
	DragMoved   bool
	DragX       int32
	DragY       int32
	// DragActivity 记录最近一次拖拽输入时间，供看门狗强制释放拖拽（见 windows.go）
	DragActivity time.Time
	// FacingRight 记录宠物当前朝向（false=向左）。walk 状态按移动方向更新，
	// render 据此对动画做水平镜像：默认向右用原帧，向左用镜像帧。
	FacingRight bool
	Anims        map[string][]*Animator
	CurrentAnim  *Animator
	Sounds       map[string][]string
	Hwnd         uintptr
	StateTimer   int
}

func (a *App) NewPet(name string) {
	a.Pet = &Pet{
		Name:    name,
		State:   "idle",
		X:       a.Cfg.WindowX,
		Y:       a.Cfg.WindowY,
		Width:   128,
		Height:  128,
		Anims:   make(map[string][]*Animator),
		Sounds:  make(map[string][]string),
		// 默认朝右（原帧），避免首帧 idle 被镜像
		FacingRight: true,
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
