//test: cmd=check
package main

func main() {
	a, b, c, d, e := 1, 2, 3, 4, 5
	println(a, b, c, d, e, a+b+c+d+e, a*b*c*d*e, a-b-c-d-e, a/b/c/d/e, a%b%c%d%e, a<<b<<c, a>>b>>c, "some long text goes here", undefinedName, "more text after the error")
}
