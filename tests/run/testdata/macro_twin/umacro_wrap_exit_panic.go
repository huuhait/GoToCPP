// The hand-written Go twin of umacro_wrap_exit_panic.gp: the code of each function moves into a func
// literal that the macro runs where it writes #body(); the results of the code are assigned to the
// results of the function.

package main

import "fmt"

func VM(gp_file string, gp_line int, gp_func string, gp_body func()) {
	fmt.Println("vm enter", gp_func)
	defer fmt.Println("vm exit", gp_func)
	gp_body()
}

func Recover(gp_file string, gp_line int, gp_func string, gp_body func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", gp_func+":", r)
		}
	}()
	gp_body()
}

func crash(n int) (gp_r0 int) {
	VM("umacro_wrap_exit_panic.gp", 26, "main.crash", func() {
		gp_r0 = func() int {
			defer fmt.Println("own defer of crash")
			if n > 0 {
				panic(fmt.Sprint("crash ", n))
			}
			return n
		}()
	})
	return
}

func contained(n int) (gp_r0 int, gp_r1 bool) {
	Recover("umacro_wrap_exit_panic.gp", 35, "main.contained", func() {
		VM("umacro_wrap_exit_panic.gp", 36, "main.contained", func() {
			gp_r0, gp_r1 = func() (int, bool) {
				defer fmt.Println("own defer of contained")
				panic(fmt.Sprint("contained ", n))
			}()
		})
	})
	return
}

func main() {
	fmt.Println(crash(0))

	func() {
		defer func() { fmt.Println("main recovered:", recover()) }()
		crash(7)
	}()

	fmt.Println(contained(3))
}
