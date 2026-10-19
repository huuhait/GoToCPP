package main

import "slices"

// box.go does not import the package of Things: the generated code names the alias as it is declared.

type Box struct {
	A     Things
	T     Table
	Names []string
}

func (b Box) Clone() Box {
	out := b
	if b.A != nil {
		out.A = slices.Clone(b.A)
	}
	if b.T != nil {
		out.T = make(Table, len(b.T))
		for k, v := range b.T {
			out.T[k] = slices.Clone(v)
		}
	}
	if b.Names != nil {
		out.Names = slices.Clone(b.Names)
	}
	return out
}

func (b Box) Equal(o Box) bool {
	if len(b.A) != len(o.A) {
		return false
	}
	for i := range b.A {
		if !b.A[i].Equal(o.A[i]) {
			return false
		}
	}
	if len(b.T) != len(o.T) {
		return false
	}
	for k, v := range b.T {
		ov, ok := o.T[k]
		if !ok || len(v) != len(ov) {
			return false
		}
		for i := range v {
			if !v[i].Equal(ov[i]) {
				return false
			}
		}
	}
	if len(b.Names) != len(o.Names) {
		return false
	}
	for i := range b.Names {
		if b.Names[i] != o.Names[i] {
			return false
		}
	}
	return true
}
