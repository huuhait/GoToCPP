// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// UserHomeDir, UserCacheDir and UserConfigDir read %USERPROFILE%, %LocalAppData% and %AppData%
// (not $HOME and the XDG variables), and say which one is not defined.
func main() {
	var h, c, cf string
	var err error
	os.Setenv("USERPROFILE", `Z:\h`)
	os.Setenv("LocalAppData", `Z:\l`)
	os.Setenv("AppData", `Z:\a`)
	h, err = os.UserHomeDir()
	fmt.Println(h, err)
	c, err = os.UserCacheDir()
	fmt.Println(c, err)
	cf, err = os.UserConfigDir()
	fmt.Println(cf, err)

	os.Unsetenv("USERPROFILE")
	os.Unsetenv("LocalAppData")
	os.Unsetenv("AppData")
	h, err = os.UserHomeDir()
	fmt.Println(h == "", e(err))
	c, err = os.UserCacheDir()
	fmt.Println(c == "", e(err))
	cf, err = os.UserConfigDir()
	fmt.Println(cf == "", e(err))
}

// Output:
// Z:\h <nil>
// Z:\l <nil>
// Z:\a <nil>
// true %userprofile% is not defined
// true %LocalAppData% is not defined
// true %AppData% is not defined
