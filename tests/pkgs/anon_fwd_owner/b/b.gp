package b

type T struct{ N int }

func (t T) Get() int   { return t.N }
func (t *T) Inc()      { t.N++ }
