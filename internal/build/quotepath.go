package build

import (
	"bytes"
	"context"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
)

// A file name with a double quote in it (a folder named q"uote, which Linux and macOS allow)
// breaks the assembler of g++: for every inline asm statement, g++ writes the line marker
//
//	# 70 "/path/q"uote/file.cpp" 1
//
// into the assembly with the name as it is, and as stops at the second quote ("unterminated
// string", "junk at end of line"). The runtime's math.cpp has an asm statement, so the build
// of the runtime in a cache folder with a quote failed, and so does a user's C++ file with asm
// in such a folder. clang escapes the name, and -ffile-prefix-map does not reach the marker.
//
// The fix keeps every path as it is: when a g++ compile has a path with a quote in it, the compile
// runs in two steps, the C++ to assembly (-S), then the assembly, with the quotes of the line
// markers escaped, to the object (-c). The dependency file comes from the first step.
//
// With -flto the marker is written at the link, from the object's intermediate code, where
// gocpp can't reach it: such a unit is compiled without LTO (a plain object that the LTO link
// takes as it is, so that unit is not optimized with the others).
//
// Only the arguments that carry a path count (the source, the folders of -I and the like, the
// forced includes): a quote in -DVER="1.0" is no reason for the second step.

// pathFlags are the flags whose value (the next word, or joined to the flag, or after an equals
// sign for the long ones) is a file or folder that the compiler reads.
var pathFlags = []string{
	"-I", "-iquote", "-isystem", "-idirafter", "-include", "-imacros", "--sysroot", "-isysroot",
}

// valueFlags take the next word as a value that is not a path of the sources (not a source file).
var valueFlags = []string{
	"-o", "-MF", "-MT", "-MQ", "-x", "-D", "-U", "-Xassembler", "-Xpreprocessor", "-Xlinker", "-aux-info",
}

// carriesQuotedPath reports whether an argument of args that names a file or folder has a
// double quote in it: a source file (a word that is not a flag and not the value of a flag), the
// value of a path flag (joined or in the next word).
func carriesQuotedPath(args []string) bool {
	for i, a := range args {
		prev := ""
		if i > 0 {
			prev = args[i-1]
		}
		switch {
		case slices.Contains(pathFlags, prev):
			if strings.Contains(a, `"`) {
				return true
			}
		case !strings.HasPrefix(a, "-"):
			if !slices.Contains(valueFlags, prev) && strings.Contains(a, `"`) {
				return true
			}
		default:
			for _, flag := range pathFlags {
				if rest, ok := strings.CutPrefix(a, flag); ok && rest != "" && strings.Contains(rest, `"`) {
					return true
				}
			}
		}
	}
	return false
}

// assemblerNeedsFix reports whether the compile c with args is one that g++'s line markers break:
// a g++ compile of one file to an object (-c ... -o file) with a double quote in a path of the
// arguments. A link, a precompiled header and -S/-E are left alone.
func assemblerNeedsFix(c CXX, args []string) bool {
	if c.Family != GCC {
		return false
	}
	if !slices.Contains(args, "-c") || slices.Contains(args, "-S") || slices.Contains(args, "-E") {
		return false
	}
	if slices.Contains(args, "c++-header") {
		return false
	}
	out := slices.Index(args, "-o")
	if out < 0 || out+1 >= len(args) {
		return false
	}
	return carriesQuotedPath(args)
}

// isLTOFlag reports whether the flag a only matters for LTO objects.
func isLTOFlag(a string) bool {
	return a == "-flto" || a == "-ffat-lto-objects" || a == "-fno-fat-lto-objects" ||
		strings.HasPrefix(a, "-flto=") || strings.HasPrefix(a, "-flto-partition=") ||
		strings.HasPrefix(a, "-flto-compression-level=")
}

// assemblerFlags returns the flags that the one-step compile hands on to the assembler, in
// their order: the target choices (-m64, -march=...), -Wa,... and -Xassembler <word>.
func assemblerFlags(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-Xassembler" && i+1 < len(args):
			out = append(out, a, args[i+1])
			i++
		case strings.HasPrefix(a, "-m") || strings.HasPrefix(a, "-Wa,"):
			out = append(out, a)
		}
	}
	return out
}

// runViaFixedAssembly is runCommand for a compile that assemblerNeedsFix chose: it compiles to
// assembly in a file next to the work folder's files, escapes the line markers, and assembles it.
// show is the command that errors name.
func (b *Builder) runViaFixedAssembly(
	ctx context.Context, c CXX, what string, link bool, args []string, show string,
) ([]byte, error) {
	asm, err := os.CreateTemp(b.Work, "asm-*.s")
	if err != nil {
		return nil, err
	}
	asm.Close()
	defer os.Remove(asm.Name())

	// Step 1: the same command without LTO, to assembly.
	plain := slices.DeleteFunc(slices.Clone(args), isLTOFlag)
	toAsm := slices.Clone(plain)
	toAsm[slices.Index(toAsm, "-c")] = "-S"
	toAsm[slices.Index(toAsm, "-o")+1] = asm.Name()
	out, err := b.execCommand(ctx, c, what, link, toAsm, show)
	if err != nil && !errors.Is(err, errOutputOpen) {
		return out, err
	}
	step1Cut := err
	data, err := os.ReadFile(asm.Name())
	if err != nil {
		return out, err
	}
	if err := os.WriteFile(asm.Name(), escapeLineMarkers(data), 0o644); err != nil {
		return out, err
	}

	// Step 2: the assembly to the object, with the flags that the assembler gets from the driver.
	// The debug information is in the assembly already.
	obj := args[slices.Index(args, "-o")+1]
	toObject := slices.Concat(
		assemblerFlags(plain),
		[]string{"-c", "-x", "assembler", asm.Name(), "-o", obj},
	)
	out2, err := b.execCommand(ctx, c, what, link, toObject, show)
	out = append(out, out2...)
	if err == nil {
		err = step1Cut
	}
	return out, err
}

// lineMarker matches the line marker that g++ writes for an inline asm statement (and for the end
// of it, with an empty name): # line "file" flag.
var lineMarker = regexp.MustCompile(`(?m)^# \d+ ".*" \d+$`)

// escapeLineMarkers returns the assembly text with the double quotes and backslashes of the file
// name in each line marker escaped, as as reads a string.
func escapeLineMarkers(asm []byte) []byte {
	return lineMarker.ReplaceAllFunc(asm, func(line []byte) []byte {
		first := bytes.IndexByte(line, '"')
		last := bytes.LastIndexByte(line, '"')
		if first == last {
			return line
		}
		name := line[first+1 : last]
		name = bytes.ReplaceAll(name, []byte(`\`), []byte(`\\`))
		name = bytes.ReplaceAll(name, []byte(`"`), []byte(`\"`))
		return slices.Concat(line[:first+1], name, line[last:])
	})
}
