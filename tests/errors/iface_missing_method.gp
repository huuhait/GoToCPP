// phase: 05

// Missing methods and methods with another signature (step 5.4).
package main

type Reader interface{ Read() string }

type NoRead struct{}

type BadRead struct{}

func (BadRead) Read() int { return 0 }

type Good struct{}

func (Good) Read() string { return "" }

func f() Reader {
	return NoRead{} // ERROR "cannot use NoRead\{\} \(value of .*type NoRead\) as Reader value in return statement: NoRead does not implement Reader \(missing method Read\)"
}

func g() Reader {
	return BadRead{} // ERROR "cannot use BadRead\{\} \(value of .*type BadRead\) as Reader value in return statement: BadRead does not implement Reader \(wrong type for method Read\)"
}

func main() {
	var r Reader = Good{}
	var a any = r
	r = a // ERROR "cannot use a \(variable of interface type any\) as Reader value in assignment: any does not implement Reader \(missing method Read\)"
	var i int = a // ERROR "cannot use a \(variable of interface type any\) as int value in variable declaration: need type assertion"
	_, _ = f(), g()
	_ = i
}
