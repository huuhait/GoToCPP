// Package store uses the methods of model in the types it has.
package store

import "example.com/derive/model"

type Shelf struct {
	model.Item
	Next   *model.Item
	Others []model.Item
	Kinds  map[model.Kind][]model.Item
	Stack  model.Stack[string]
}

func (s Shelf) Equal(o Shelf) bool {
	if !s.Item.Equal(o.Item) {
		return false
	}
	if s.Next != o.Next {
		if s.Next == nil || o.Next == nil || !s.Next.Equal(*o.Next) {
			return false
		}
	}
	if len(s.Others) != len(o.Others) {
		return false
	}
	for i := range s.Others {
		if !s.Others[i].Equal(o.Others[i]) {
			return false
		}
	}
	if len(s.Kinds) != len(o.Kinds) {
		return false
	}
	for k, v := range s.Kinds {
		ov, ok := o.Kinds[k]
		if !ok || len(v) != len(ov) {
			return false
		}
		for i := range v {
			if !v[i].Equal(ov[i]) {
				return false
			}
		}
	}
	return s.Stack.Equal(o.Stack)
}

func (s Shelf) Clone() Shelf {
	out := s
	out.Item = s.Item.Clone()
	if s.Next != nil {
		c := s.Next.Clone()
		out.Next = &c
	}
	if s.Others != nil {
		out.Others = make([]model.Item, len(s.Others))
		for i := range s.Others {
			out.Others[i] = s.Others[i].Clone()
		}
	}
	if s.Kinds != nil {
		out.Kinds = make(map[model.Kind][]model.Item, len(s.Kinds))
		for k, v := range s.Kinds {
			var cv []model.Item
			if v != nil {
				cv = make([]model.Item, len(v))
				for i := range v {
					cv[i] = v[i].Clone()
				}
			}
			out.Kinds[k] = cv
		}
	}
	out.Stack = s.Stack.Clone()
	return out
}
