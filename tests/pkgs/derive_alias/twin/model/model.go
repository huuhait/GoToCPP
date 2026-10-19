package model

type Thing struct {
	N    int
	Name string
}

func (t Thing) Equal(o Thing) bool { return t.N == o.N }

type Tags []string
