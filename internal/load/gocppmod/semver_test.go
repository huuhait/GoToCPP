package gocppmod

import "testing"

func TestCompareVersionsOrder(t *testing.T) {
	// Each version is lower than every one that follows (semver.org rule 11, Go's
	// pseudo-versions, and numbers longer than a machine word).
	ordered := []string{
		"v0.0.0-20200101000000-aaaaaaaaaaaa",
		"v0.0.0-20210101000000-bbbbbbbbbbbb",
		"v0.0.0",
		"v0.1.0",
		"v1.0.0-alpha",
		"v1.0.0-alpha.1",
		"v1.0.0-alpha.beta",
		"v1.0.0-beta",
		"v1.0.0-beta.2",
		"v1.0.0-beta.11",
		"v1.0.0-rc.1",
		"v1.0.0",
		"v1.0.1-0.20200101000000-aaaaaaaaaaaa",
		"v1.0.1",
		"v1.1.0",
		"v1.2.0",
		"v1.9.0",
		"v1.10.0",
		"v1.11.0",
		"v2.0.0",
		"v10.0.0",
		"v99999999999999999999.0.0",
		"v100000000000000000000.0.0",
	}
	for i, a := range ordered {
		if !ValidVersion(a) {
			t.Errorf("%s is not valid", a)
		}
		for j, b := range ordered {
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got := CompareVersions(a, b); got != want {
				t.Errorf("CompareVersions(%s, %s) = %d, want %d", a, b, got, want)
			}
		}
	}
}

func TestCompareVersionsBuildMetadataAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"v2.0.0+incompatible", "v2.0.0", 0},
		{"v2.0.0", "v2.0.0+incompatible", 0},
		{"v1.9.0", "v2.0.0+incompatible", -1},
		{"v2.0.0+incompatible", "v2.0.1", -1},
		{"", "v0.0.0", -1},
		{"none", "v0.0.0", -1},
		{"v0.0.0", "none", 1},
		{"none", "junk", 1}, // two invalid versions compare as strings
		{"", "", 0},
		{"v1.2", "v1.2.0", -1}, // not a full version
	} {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestVersionHelpers(t *testing.T) {
	if !IsPrerelease("v1.0.0-rc.1") || !IsPrerelease("v0.0.0-20200101000000-aaaaaaaaaaaa") || IsPrerelease("v1.0.0") || IsPrerelease("v1.0.0+incompatible") || IsPrerelease("junk") {
		t.Error("IsPrerelease")
	}
	if Major("v2.3.4") != "2" || Major("v0.0.1-x") != "0" || Major("x") != "" {
		t.Error("Major")
	}
	if got := CanonicalVersion("v1.2.3+meta"); got != "v1.2.3" {
		t.Errorf("canonical = %s", got)
	}
	if got := CanonicalVersion("v2.0.0+incompatible"); got != "v2.0.0+incompatible" {
		t.Errorf("canonical = %s", got)
	}
	for _, bad := range []string{"", "1.2.3", "v1", "v1.2", "v01.2.3", "v1.2.3-01", "v1.2.3-", "v1.2.3+", "v1.2.3-a..b"} {
		if ValidVersion(bad) {
			t.Errorf("%q should not be valid", bad)
		}
	}
}
