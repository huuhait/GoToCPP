package engine

type Body struct{ Mass float64 }

type ID int

func New() *Body { return &Body{Mass: 1} }
