package base

type Getter interface{ Get() int }

type Incer interface{ Inc() }

type Base struct{ n int }

func New(n int) Base { return Base{n} }

func (b Base) Get() int { return b.n }
func (b *Base) Inc()    { b.n++ }
