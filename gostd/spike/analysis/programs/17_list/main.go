// container/list: New, PushBack, PushFront, InsertBefore/After, Len, iteration with Next,
// MoveToFront/Back/Before, and Remove. This is the first of the four spike packages.
package main

import "container/list"

func main() {
	l := list.New()
	e4 := l.PushBack(4)
	l.PushFront(1)
	e3 := l.InsertAfter(3, e4)
	l.InsertBefore(2, e3)
	println(l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		println(e.Value.(int))
	}
	l.MoveToFront(e3)
	l.MoveToBack(e4)
	l.MoveBefore(e4, l.Front())
	l.Remove(e3)
	println(l.Len(), l.Front().Value.(int), l.Back().Value.(int))
}
