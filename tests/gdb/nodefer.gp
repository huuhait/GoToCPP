// phase: 05

// gp catch panic in a program without defers: the runtime has no gp_panic (the linker dropped it),
// so the command says so, and the throw of the panic still stops with the Go function as the
// selected frame. The session is nodefer.gdb.
package main

func boom() {
	panic("plain") // @boom
}

func main() {
	boom()
}
