// phase: 05

// A value of T does not implement an interface whose method has a pointer receiver; the message
// is go/types' (step 5.4).
package main

type Shape interface{ Scale(k float64) }

type Rect struct{ w, h float64 }

func (r *Rect) Scale(k float64) { r.w *= k; r.h *= k }

func use(s Shape) {}

func main() {
	var r Rect
	var s Shape = r // ERROR "cannot use r \(variable of .*type Rect\) as Shape value in variable declaration: Rect does not implement Shape \(method Scale has pointer receiver\)"
	use(r)          // ERROR "cannot use r \(variable of .*type Rect\) as Shape value in argument to use: Rect does not implement Shape \(method Scale has pointer receiver\)"
	use(&r)
	use(Rect{})  // ERROR "cannot use Rect\{\} \(value of .*type Rect\) as Shape value in argument to use: Rect does not implement Shape \(method Scale has pointer receiver\)"
	_ = []Shape{r} // ERROR "cannot use r \(variable of .*type Rect\) as Shape value in array or slice literal: Rect does not implement Shape \(method Scale has pointer receiver\)"
	_ = s
}
