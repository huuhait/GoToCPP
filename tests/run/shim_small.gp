// phase: 09
// std-internal: tests the small internal packages, which only the standard library may import

// The small internal packages the ported standard library imports: itoa (integers to text without
// strconv), oserror (the error values of os), goarch and goos (the target's constants), cpu (a stub
// whose features are all false), abi (NoEscape and Escape return their argument), race (no-ops) and
// testlog (the logger that os reports to).
package main

import (
	"errors"
	"fmt"
	"internal/abi"
	"internal/cpu"
	"internal/goarch"
	"internal/goos"
	"internal/itoa"
	"internal/oserror"
	"internal/race"
	"internal/testlog"
	"unsafe"
)

type logger struct{ events []string }

func (l *logger) Getenv(key string) { l.events = append(l.events, "getenv "+key) }
func (l *logger) Stat(file string)  { l.events = append(l.events, "stat "+file) }
func (l *logger) Open(file string)  { l.events = append(l.events, "open "+file) }
func (l *logger) Chdir(dir string)  { l.events = append(l.events, "chdir "+dir) }

func main() {
	// itoa
	sep := ""
	for _, n := range []int{0, 1, -1, 9, 10, 99, 100, 123456789, -123456789, -1 << 63, 1 << 62, -(1 << 62), 9223372036854775807, -9223372036854775807} {
		fmt.Printf("%sItoa(%d)=%s", sep, n, itoa.Itoa(n))
		sep = " "
	}
	fmt.Println()
	fmt.Println(itoa.Uitoa(0), itoa.Uitoa(18446744073709551615), itoa.Uitox(0), itoa.Uitox(255), itoa.Uitox(0xdeadbeef), itoa.Uitox(18446744073709551615))

	// oserror: distinct values with Go's texts, usable with errors.Is
	for _, e := range []error{oserror.ErrInvalid, oserror.ErrPermission, oserror.ErrExist, oserror.ErrNotExist, oserror.ErrClosed} {
		fmt.Println(e, errors.Is(e, oserror.ErrExist), errors.Is(fmt.Errorf("wrap: %w", e), e))
	}

	// goarch and goos: the constants of the target; the checks are of what must hold on any target
	fmt.Println(goarch.PtrSize, goarch.BigEndian, goarch.GOARCH == "amd64" || goarch.GOARCH == "arm64")
	fmt.Println(goarch.IsAmd64+goarch.IsArm64, goarch.Is386+goarch.IsArm+goarch.IsWasm+goarch.IsRiscv64)
	fmt.Println(goos.GOOS == "linux" || goos.GOOS == "windows", goos.IsLinux+goos.IsWindows, goos.IsUnix == (goos.IsLinux == 1))
	fmt.Println(goarch.PCQuantum > 0, goarch.StackAlign > 0, goarch.DefaultPhysPageSize >= 4096)

	// cpu: every feature is false, so no ported package takes a processor-specific path
	fmt.Println(cpu.X86.HasAVX2, cpu.X86.HasFMA, cpu.X86.HasPOPCNT, cpu.ARM64.HasCRC32, cpu.DebugOptions, cpu.Name() == "")
	fmt.Println(cpu.CacheLinePadSize == 64 || cpu.CacheLinePadSize == 128, uintptr(cpu.CacheLineSize) == uintptr(cpu.CacheLinePadSize))
	var pad cpu.CacheLinePad
	fmt.Println(unsafe.Sizeof(pad) == cpu.CacheLinePadSize)

	// abi: the identity
	x := 42
	p := unsafe.Pointer(&x)
	fmt.Println(abi.NoEscape(p) == p, *(*int)(abi.NoEscape(p)))
	fmt.Println(abi.Escape(7), abi.Escape("s"), abi.Escape([]int{1, 2})[1])

	// race: nothing happens
	race.Acquire(p)
	race.Release(p)
	race.ReleaseMerge(p)
	race.Read(p)
	race.Write(p)
	race.ReadRange(p, 8)
	race.WriteRange(p, 8)
	race.Disable()
	race.Enable()
	fmt.Println(race.Enabled, race.Errors())

	// testlog: no logger, then one; a second SetLogger panics
	testlog.Getenv("X")
	testlog.Open("a")
	testlog.Stat("b")
	fmt.Println(testlog.Logger() == nil, testlog.PanicOnExit0())
	testlog.SetPanicOnExit0(true)
	fmt.Println(testlog.PanicOnExit0())
	testlog.SetPanicOnExit0(false)
	l := &logger{}
	testlog.SetLogger(l)
	testlog.Getenv("HOME")
	testlog.Open("/etc/passwd")
	testlog.Stat("/tmp")
	testlog.Logger().Chdir("/")
	fmt.Println(l.events, testlog.Logger() == testlog.Interface(l))
	func() {
		defer func() { fmt.Println(recover()) }()
		testlog.SetLogger(&logger{})
	}()
}

// Output:
// Itoa(0)=0 Itoa(1)=1 Itoa(-1)=-1 Itoa(9)=9 Itoa(10)=10 Itoa(99)=99 Itoa(100)=100 Itoa(123456789)=123456789 Itoa(-123456789)=-123456789 Itoa(-9223372036854775808)=-9223372036854775808 Itoa(4611686018427387904)=4611686018427387904 Itoa(-4611686018427387904)=-4611686018427387904 Itoa(9223372036854775807)=9223372036854775807 Itoa(-9223372036854775807)=-9223372036854775807
// 0 18446744073709551615 0x0 0xff 0xdeadbeef 0xffffffffffffffff
// invalid argument false true
// permission denied false true
// file already exists true true
// file does not exist false true
// file already closed false true
// 8 false true
// 1 0
// true 1 true
// true true true
// false false false false false true
// true true
// true
// true 42
// 7 s 2
// false 0
// true false
// true
// [getenv HOME open /etc/passwd stat /tmp chdir /] true
// testlog: SetLogger must be called only once
