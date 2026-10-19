package lib

import "fmt"

type Entity[ID comparable] struct {
	ID   ID
	tags []string
}

func (e Entity[ID]) Describe() string { return fmt.Sprintf("entity %v %v", e.ID, e.tags) }
func (e *Entity[ID]) Tag(t string)   { e.tags = append(e.tags, t) }

type Describer interface{ Describe() string }

type Repo[ID comparable, T any] struct {
	items map[ID]T
}

func NewRepo[ID comparable, T any]() *Repo[ID, T] { return &Repo[ID, T]{items: map[ID]T{}} }

func (r *Repo[ID, T]) Save(id ID, v T) { r.items[id] = v }
func (r *Repo[ID, T]) Find(id ID) (T, bool) {
	v, ok := r.items[id]
	return v, ok
}
