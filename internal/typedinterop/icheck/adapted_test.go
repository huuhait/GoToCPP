package icheck

import "testing"

func TestIntegerLiteral(t *testing.T) {
	for text, want := range map[string]bool{
		"2": true, "0x4": true, "0X1F": true, "-3": true, "+7": true, "1e3": false, "2.5": false,
		"flagMask": false, "": false, "'a'": false, "0": true,
	} {
		if got := integerLiteral(text); got != want {
			t.Errorf("integerLiteral(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestIsWideCharPointer(t *testing.T) {
	for typ, want := range map[string]bool{
		"const wchar_t *": true, "wchar_t *": true, "const char16_t *": true, "const char32_t *": false,
		"const char *": false, "const wchar_t **": false, "wchar_t": false, "const unsigned short *": false,
	} {
		if got := isWideCharPointer(typ); got != want {
			t.Errorf("isWideCharPointer(%q) = %v, want %v", typ, got, want)
		}
	}
}
