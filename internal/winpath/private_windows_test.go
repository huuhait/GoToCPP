//go:build windows

package winpath

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The private folder, on the real PC (tests/windows_native_test.go runs this package's tests there). The ACLs are read back
// with icacls, which is independent of the code under test, and with VerifyPrivate.

func icacls(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("icacls", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("icacls %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestPrivateFolderACL(t *testing.T) {
	base := t.TempDir()
	dir, err := PrivateFolder(base, "abc123", "TEMP")
	if err != nil {
		t.Fatal(err)
	}
	userStr, _, _ := currentUser()
	want := filepath.Join(base, "gocpp-paths-"+userStr, "abc123", "TEMP")
	if dir != want {
		t.Fatalf("PrivateFolder = %q, want %q", dir, want)
	}
	// Every level: no entry for Users, Everyone or Authenticated Users, SYSTEM and Administrators have one, and the top one is protected
	// (nothing inherited from the parent: no "(I)" entry).
	top := filepath.Join(base, "gocpp-paths-"+userStr)
	for _, d := range []string{top, filepath.Join(top, "abc123"), dir} {
		out := icacls(t, d)
		for _, bad := range []string{`BUILTIN\Users`, `Everyone`, `Authenticated Users`, `CREATOR OWNER`} {
			if strings.Contains(out, bad) {
				t.Errorf("%s has an entry for %s:\n%s", d, bad, out)
			}
		}
		for _, good := range []string{`NT AUTHORITY\SYSTEM:(OI)(CI)(F)`, `BUILTIN\Administrators:(OI)(CI)(F)`} {
			if !strings.Contains(out, good) {
				t.Errorf("%s lacks %s:\n%s", d, good, out)
			}
		}
	}
	if out := icacls(t, top); strings.Contains(out, "(I)") {
		t.Errorf("the top folder inherits entries:\n%s", out)
	}
	if err := VerifyPrivate(dir); err != nil {
		t.Errorf("VerifyPrivate: %v", err)
	}
	// Again: the same folder, no error.
	if again, err := PrivateFolder(base, "abc123", "TEMP"); err != nil || again != dir {
		t.Errorf("second call = %q, %v", again, err)
	}
	// A file made in it can be used.
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Error(err)
	}
}

// A folder that is there already is refused when someone else has access to it, or owns it, or when it is a link.
func TestPrivateFolderRefusesWhatIsNotPrivate(t *testing.T) {
	userStr, _, _ := currentUser()
	t.Run("opened ACL", func(t *testing.T) {
		base := t.TempDir()
		top := filepath.Join(base, "gocpp-paths-"+userStr)
		if _, err := PrivateFolder(base); err != nil { // made by us: private
			t.Fatal(err)
		}
		icacls(t, top, "/grant", "*S-1-1-0:(OI)(CI)F") // Everyone: full control
		if _, err := PrivateFolder(base); err == nil || !strings.Contains(err.Error(), "gives access") {
			t.Errorf("a folder that Everyone can write: error = %v", err)
		}
		if err := VerifyPrivate(top); err == nil {
			t.Error("VerifyPrivate accepted it")
		}
	})
	t.Run("users can read", func(t *testing.T) {
		base := t.TempDir()
		top := filepath.Join(base, "gocpp-paths-"+userStr)
		if _, err := PrivateFolder(base); err != nil {
			t.Fatal(err)
		}
		icacls(t, top, "/grant", "*S-1-5-32-545:(OI)(CI)RX") // BUILTIN\Users: read and execute (what ProgramData gives by inheritance)
		if _, err := PrivateFolder(base); err == nil {
			t.Error("a folder that all users can read was accepted")
		}
	})
	// One access right for another principal is enough to refuse the folder, each of the bits in anyAccess (FILE_READ_DATA alone, ...). The
	// rights that change nothing a user could abuse (read attributes, read the ACL, synchronize) are not refused.
	t.Run("a single right", func(t *testing.T) {
		for _, right := range []string{"RD", "WD", "AD", "REA", "WEA", "X", "DC", "WA", "D", "WDAC", "WO"} {
			base := t.TempDir()
			top := filepath.Join(base, "gocpp-paths-"+userStr)
			if _, err := PrivateFolder(base); err != nil {
				t.Fatal(err)
			}
			icacls(t, top, "/grant", "*S-1-5-32-545:(OI)(CI)("+right+")")
			if _, err := PrivateFolder(base); err == nil || !strings.Contains(err.Error(), "gives access") {
				t.Errorf("Users with only %s: error = %v", right, err)
			}
		}
		for _, right := range []string{"RA", "RC", "S"} {
			base := t.TempDir()
			top := filepath.Join(base, "gocpp-paths-"+userStr)
			if _, err := PrivateFolder(base); err != nil {
				t.Fatal(err)
			}
			icacls(t, top, "/grant", "*S-1-5-32-545:(OI)(CI)("+right+")")
			if _, err := PrivateFolder(base); err != nil {
				t.Errorf("Users with only %s (nothing to read or write) refused: %v", right, err)
			}
		}
	})
	t.Run("other owner", func(t *testing.T) {
		base := t.TempDir()
		top := filepath.Join(base, "gocpp-paths-"+userStr)
		if _, err := PrivateFolder(base); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("icacls", top, "/setowner", "*S-1-5-32-545").CombinedOutput() // BUILTIN\Users
		if err != nil {
			t.Skipf("cannot set another owner here: %v\n%s", err, out)
		}
		if _, err := PrivateFolder(base); err == nil || !strings.Contains(err.Error(), "is owned by") {
			t.Errorf("a folder owned by Users: error = %v", err)
		}
	})
	t.Run("a link", func(t *testing.T) {
		base := t.TempDir()
		target := t.TempDir()
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(base, "gocpp-paths-"+userStr), target).CombinedOutput(); err != nil {
			t.Skipf("no junction: %v\n%s", err, out)
		}
		if _, err := PrivateFolder(base); err == nil || !strings.Contains(err.Error(), "link") {
			t.Errorf("a junction: error = %v", err)
		}
	})
}

// The point of the flag WC_NO_BEST_FIT_CHARS: Ł has a best-fit counterpart (L) in code page 1252, which is a different file name.
func TestACPSafeNoBestFit(t *testing.T) {
	if acp, _, _ := procGetACP.Call(); acp != 1252 {
		t.Skipf("code page %d: Ł is in some code pages", acp)
	}
	for _, s := range []string{"\u0141", "tmp_\u0141", "C:\\Users\\\u0141ukasz\\x", "\u03a9"} { // Ł, Ω (best fit: O)
		if ACPSafe(s) {
			t.Errorf("ACPSafe(%q) = true in code page 1252", s)
		}
	}
	for _, s := range []string{"\u00e9", "J\u00fcrgen", "plain"} {
		if !ACPSafe(s) {
			t.Errorf("ACPSafe(%q) = false in code page 1252", s)
		}
	}
}
