package inner

type Base struct{ N int }

func (b Base) Get() int  { return b.N }
func (b *Base) Add(d int) { b.N += d }
