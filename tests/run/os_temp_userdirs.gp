// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	os.Setenv("HOME", "/home/test")
	os.Setenv("XDG_CACHE_HOME", "")
	os.Setenv("XDG_CONFIG_HOME", "")
	fmt.Println(os.UserHomeDir())
	fmt.Println(os.UserCacheDir())
	fmt.Println(os.UserConfigDir())
	os.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	os.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	fmt.Println(os.UserCacheDir())
	fmt.Println(os.UserConfigDir())
	os.Setenv("XDG_CACHE_HOME", "relative")
	fmt.Println(os.UserCacheDir())
	os.Setenv("HOME", "")
	os.Setenv("XDG_CONFIG_HOME", "")
	fmt.Println(os.UserHomeDir())
	fmt.Println(os.UserConfigDir())
}

// Output:
// /home/test <nil>
// /home/test/.cache <nil>
// /home/test/.config <nil>
// /xdg/cache <nil>
// /xdg/config <nil>
//  path in $XDG_CACHE_HOME is relative
//  $HOME is not defined
//  neither $XDG_CONFIG_HOME nor $HOME are defined
