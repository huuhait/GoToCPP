package a

// Holder keeps unnamed interface types by value in its fields, and no function of this package
// ever assigns into them.
type Holder struct {
	Mu    interface{ Lock() }
	Items [2]interface{ Get() int }
	Cb    func(interface{ Get() int }) int
}

type Num int

func (n Num) Get() int { return int(n) }

func New() Holder { return Holder{} }
