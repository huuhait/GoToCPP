package envflag

import "testing"

func TestOffline(t *testing.T) {
	for v, want := range map[string]bool{
		"": false, "0": false, "false": false, "no": false, "off": false,
		"False": false, "OFF": false, " off ": false, " ": false, "No": false,
		"1": true, "true": true, "yes": true, "on": true, "TRUE": true, "On": true, " 1 ": true,
		"2": true, "y": true, "ture": true, "enabled": true,
	} {
		if got := Offline(v); got != want {
			t.Errorf("Offline(%q) = %v, want %v", v, got, want)
		}
	}
}
