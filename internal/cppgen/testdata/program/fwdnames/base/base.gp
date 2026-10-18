package base

type Base struct{ N string }

func (b Base) name() string { return "base:" + b.N }

func (b Base) Hello() string { return b.name() }
