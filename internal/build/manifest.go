package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/huuhait/gocpp/internal/fsretry"
	"github.com/huuhait/gocpp/internal/winpath"
)

// The application manifest of a Windows program (Phase 14 step 14.2, ADR-081). Windows reads an XML
// resource of the .exe that says how the program wants the system to treat it:
//
//   - longPathAware: file names longer than 260 characters work with the plain Win32 calls, also
//     SetCurrentDirectory (os.Chdir). Go's own programs have no manifest, so for them only the calls
//     that Go's os package prefixes with \\?\ take long names; with the manifest (and the system's
//     LongPathsEnabled switch, on by default in Windows 11 Pro's developer setups and in Windows
//     Server 2022+, off in a plain Windows 10) the C and C++ code of a program does too. The system
//     decides; the manifest only agrees.
//   - activeCodePage UTF-8 (Windows 10 1903 and later): the narrow "A" functions and the C library's
//     file names (fopen, std::ifstream, std::filesystem) read their char strings as UTF-8, which is what
//     every string of a gocpp program is. Without it a Go string with a non-ASCII file name, handed to
//     fopen, is read in the system's ANSI code page and names another file.
//   - requestedExecutionLevel asInvoker: without a manifest Windows guesses from the file name and
//     asks for administrator rights for a program called setup.exe, install.exe, update.exe or patch.exe.
//   - supportedOS Windows 10: the version functions tell the truth.
//
// Go's linker writes no manifest. The resource is linked in as one small COFF object that gocpp writes
// itself (manifestObject), so that no resource compiler (windres) is needed and every toolchain's
// linker, GNU ld and lld, takes it the same way as a windres object: two sections, .rsrc$01 (the
// directory tree) and .rsrc$02 (the data), with one relocation for the data's address.

// manifestXML is the manifest.
const manifestXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <longPathAware xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">true</longPathAware>
      <activeCodePage xmlns="http://schemas.microsoft.com/SMI/2019/WindowsSettings">UTF-8</activeCodePage>
    </windowsSettings>
  </application>
</assembly>
`

// manifestObject returns a COFF object file (x86-64) whose resource section holds the manifest xml as
// the resource RT_MANIFEST (24), id 1 (CREATEPROCESS_MANIFEST_RESOURCE_ID), language 0.
func manifestObject(xml string) []byte {
	le := binary.LittleEndian
	// Section .rsrc$01: three directories of one entry each (type, name, language) and the data entry.
	var tree bytes.Buffer
	dir := func(id uint32, sub uint32) {
		binary.Write(&tree, le, struct {
			Characteristics, TimeDateStamp uint32
			Major, Minor, Named, IDs       uint16
		}{0, 0, 0, 0, 0, 1})
		binary.Write(&tree, le, [2]uint32{id, sub})
	}
	dir(24, 0x80000000|24) // type RT_MANIFEST -> directory at 24
	dir(1, 0x80000000|48)  // name 1 -> directory at 48
	dir(0, 72)             // language 0 -> the data entry at 72
	const dataEntryAt = 72
	binary.Write(&tree, le, [4]uint32{0, uint32(len(xml)), 0, 0}) // RVA of the data (relocated), size, code page, reserved
	sec1 := tree.Bytes()
	sec2 := []byte(xml)
	for len(sec2)%4 != 0 {
		sec2 = append(sec2, 0)
	}

	const (
		fileHeader = 20
		secHeader  = 40
	)
	off1 := fileHeader + 2*secHeader
	off2 := off1 + len(sec1)
	offReloc := off2 + len(sec2)
	offSyms := offReloc + 10
	const flags = 0x40000040 | 0x00300000 // initialized data, readable, 4-byte alignment
	var b bytes.Buffer
	w := func(v any) { binary.Write(&b, le, v) }
	// File header: AMD64, two sections, the symbol table after the relocation.
	w(struct {
		Machine, NumberOfSections        uint16
		TimeDateStamp, PointerToSymbols  uint32
		NumberOfSymbols                  uint32
		SizeOfOptionalHeader, Characters uint16
	}{0x8664, 2, 0, uint32(offSyms), 4, 0, 0})
	name := func(s string) [8]byte {
		var n [8]byte
		copy(n[:], s)
		return n
	}
	type section struct {
		Name                           [8]byte
		VirtualSize, VirtualAddress    uint32
		SizeOfRawData, PointerToRaw    uint32
		PointerToRelocs, PointerToLine uint32
		NumberOfRelocs, NumberOfLine   uint16
		Characteristics                uint32
	}
	w(section{name(".rsrc$01"), 0, 0, uint32(len(sec1)), uint32(off1), uint32(offReloc), 0, 1, 0, flags})
	w(section{name(".rsrc$02"), 0, 0, uint32(len(sec2)), uint32(off2), 0, 0, 0, 0, flags})
	b.Write(sec1)
	b.Write(sec2)
	// The relocation: the data entry's first field is the address of .rsrc$02 (IMAGE_REL_AMD64_ADDR32NB).
	w(struct {
		VirtualAddress, SymbolIndex uint32
		Type                        uint16
	}{dataEntryAt, 2, 3}) // symbol 2: the section symbol of .rsrc$02 (symbol 0 has an auxiliary record, which is entry 1)
	// Symbols: a section symbol for each section, with an auxiliary record each (static, class 3).
	type symbol struct {
		Name          [8]byte
		Value         uint32
		Section       int16
		Type          uint16
		Class, NumAux uint8
	}
	type aux struct {
		Length        uint32
		Relocs, Lines uint16
		Checksum      uint32
		Number        uint16
		Selection     uint8
		_             [3]byte
	}
	w(symbol{name(".rsrc$01"), 0, 1, 0, 3, 1})
	w(aux{Length: uint32(len(sec1)), Relocs: 1})
	w(symbol{name(".rsrc$02"), 0, 2, 0, 3, 1})
	w(aux{Length: uint32(len(sec2))})
	w(uint32(4)) // the string table: its own size, no long names
	return b.Bytes()
}

// manifestFile writes the manifest object as dir/name (a folder of the cache, or the work folder) and
// returns the path: the same bytes for every build, written once (not written when write is false: a dry run).
func manifestFile(dir, name string, write bool) (string, error) {
	data := manifestObject(manifestXML)
	p := filepath.Join(dir, name)
	if !write {
		return p, nil
	}
	if have, err := os.ReadFile(p); err == nil && bytes.Equal(have, data) {
		return p, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".manifest-*")
	if err != nil {
		return "", err
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return "", werr
	}
	if err := fsretry.Rename(tmp.Name(), p); err != nil {
		os.Remove(tmp.Name())
		if have, rerr := os.ReadFile(p); rerr == nil && bytes.Equal(have, data) {
			return p, nil
		}
		return "", err
	}
	return p, nil
}

// defaultManifestName is the file that MinGW-w64's GCC links into every program by itself: its specs say
// %{!shared:%:if-exists(default-manifest.o%s)}, a manifest of its own (trustInfo and nothing else). Two
// manifests in one program are an error that GNU ld does not report: it keeps the first and leaves
// the other as unreadable garbage in the resource section ("Extra data in .rsrc section"). So with GCC the
// manifest is not one more object: gocpp's object is a file of that name in a folder of its own, and -B
// puts the folder before GCC's, so the compiler driver links ours *in the place of* its default.
// clang (llvm-mingw, MSYS2's clang64) links no default manifest and takes the object as an input file.
const defaultManifestName = "default-manifest.o"

// manifestLink returns what a Windows program's link needs for the manifest of compiler family f: an
// object file for the link line (clang), or a driver flag (GCC's -B), or neither and a warning when GCC has no folder
// it can be told about.
//
// A GCC -B prefix is expanded into GCC's own spec text (%:if-exists(default-manifest.o%s)), which is split at spaces and read
// in the ANSI code page, so the folder must be an absolute path of ASCII characters without a space (a relative one is
// not looked at by the spec function: measured with GCC 16.1). The cache is often not such a folder
// (C:\Users\John Smith\AppData\Local\gocpp: "cannot find C:/Users/John", found by the review). So the candidates are, in
// order: the cache folder; its 8.3 short name; the work folder (and its short name); a folder below ProgramData
// (programData: %ProgramData%\gocpp-paths-<SID>\manifest-<hash>, private to the user: winpath.PrivateFolder).
// The first that is plain and can be written is used.
func manifestLink(f Family, cacheDir, workDir, programData string, write bool) (obj, flag, warning string, err error) {
	sum := sha256.Sum256(manifestObject(manifestXML))
	hash := hex.EncodeToString(sum[:6])
	if f != GCC {
		base := cacheDir
		if base == "" {
			base = workDir
		}
		p, err := manifestFile(filepath.Join(base, "manifest", hash), "manifest.o", write)
		return p, "", "", err
	}
	var candidates []string
	for _, base := range []string{cacheDir, workDir} {
		if base != "" {
			candidates = append(candidates, filepath.Join(base, "manifest", hash))
		}
	}
	for _, dir := range candidates {
		if !plainFolder(dir) {
			if write {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					continue
				}
			}
			short, ok := winpath.Short(dir)
			if !ok || !plainFolder(short) {
				continue
			}
			dir = short
		}
		if _, err := manifestFile(dir, defaultManifestName, write); err != nil {
			continue
		}
		return "", "-B" + filepath.ToSlash(dir) + "/", "", nil
	}
	// A private folder below ProgramData (shared place: see winpath.PrivateFolder for what private means; the one folder of this
	// user, with a protected ACL, whose owner is checked when it exists).
	refused := ""
	if programData != "" && write {
		dir, err := privateFolder(programData, "manifest-"+hash)
		if err == nil && plainFolder(dir) {
			if _, err = manifestFile(dir, defaultManifestName, true); err == nil {
				return "", "-B" + filepath.ToSlash(dir) + "/", "", nil
			}
		}
		if err != nil {
			refused = " (the private folder below ProgramData could not be used: " + err.Error() + ")"
		}
	}
	if !write {
		return "", "", "", nil // a dry run prints the commands; the warning is for the real link
	}
	return "", "", "no folder without spaces or non-ASCII characters could hold the application manifest, so this program is built without it (MinGW's own default manifest is used; set GOCPP_NO_MANIFEST=1 to silence this)" + refused, nil
}

// privateFolder makes a private folder below a shared place (winpath.PrivateFolder); the tests replace it.
var privateFolder = winpath.PrivateFolder

// plainFolder reports whether the folder name is one that GCC's -B can carry: absolute, ASCII, and without a space, tab, quote or %.
func plainFolder(p string) bool {
	if !filepath.IsAbs(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if c := p[i]; c >= 0x80 || c <= ' ' || c == '"' || c == '\'' || c == '%' {
			return false
		}
	}
	return true
}
