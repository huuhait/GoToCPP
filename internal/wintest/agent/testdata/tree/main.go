// Command tree is a test program of the Windows suite's agent (tests/windows_suite_agent_test.go), built with
// GOOS=windows: "hang" starts a child that sleeps forever and then sleeps itself; "leak" starts such a child
// and exits at once; "flood" prints without end; "sleep" sleeps ten minutes; "exit7" exits with 7; "tmp" prints TEMP and the working folder.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	switch os.Args[1] {
	case "child":
		time.Sleep(time.Hour)
	case "sleep":
		fmt.Println("sleeping")
		time.Sleep(10 * time.Minute)
	case "hang", "leak":
		c := exec.Command(os.Args[0], "child")
		if err := c.Start(); err != nil {
			fmt.Println("start failed:", err)
			os.Exit(1)
		}
		fmt.Println("started")
		if os.Args[1] == "hang" {
			time.Sleep(time.Hour)
		}
	case "flood":
		line := strings.Repeat("x", 1023) + "\n"
		for {
			fmt.Print(line)
		}
	case "exit7":
		os.Exit(7)
	case "tmp":
		wd, _ := os.Getwd()
		fmt.Println(os.Getenv("TEMP"))
		fmt.Println(wd)
	}
}
