package main

import (
	"testing"

	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/toolchain"
)

// TestOfflineSpellingsAgree: the toolchain download and the module download read GOCPP_OFFLINE
// the same way, for every spelling (they used to differ: off was on for the toolchain and off for
// the modules; yes was on for both, y was on for one).
func TestOfflineSpellingsAgree(t *testing.T) {
	for _, v := range []string{
		"", " ", "0", "false", "False", "no", "NO", "off", "Off", " off ",
		"1", "true", "TRUE", "yes", "Yes", "on", "ON", " 1 ", "2", "y", "ture", "enabled",
	} {
		t.Setenv("GOCPP_OFFLINE", v)
		t.Setenv("GOCPP_MODCACHE", "off")
		tc, err := toolchain.FromEnv(true)
		if err != nil {
			t.Fatal(err)
		}
		mod, err := modfetch.ConfigFromEnv(func(k string) string {
			if k == "GOCPP_OFFLINE" {
				return v
			}
			if k == "GOCPP_MODCACHE" {
				return "off"
			}
			return ""
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if tc.Offline != mod.Offline {
			t.Errorf("GOCPP_OFFLINE=%q: toolchain Offline = %v, modules Offline = %v", v, tc.Offline, mod.Offline)
		}
	}
}
