package main

// gocpp debug -vscode (Phase 14, step 14.6): the launch.json that starts a gocpp program under gdb
// or lldb from VS Code, for Linux and for Windows. docs/guides/debugging.md shows the same files.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	rtfiles "github.com/huuhait/gocpp/runtime"
)

// vscodeFolder is the folder of the workspace in VS Code's variables.
const vscodeFolder = "${workspaceFolder}"

// lldbScriptName is the file that -vscode -debugger lldb writes to the workspace's .vscode folder:
// the summaries for lldb, which a launch configuration loads (initCommands) every time.
const lldbScriptName = "gocpp_lldb.py"

// vscodeProgram returns the program's name for the launch file: the .gp file or package that the
// command line names (hello.gp is hello, ./cmd/tool is tool), else the folder's name.
func vscodeProgram(args []string, dir string) string {
	name := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name = strings.TrimSuffix(filepath.Base(filepath.ToSlash(args[0])), ".gp")
	}
	if name == "" || name == "." || name == ".." || name == "/" {
		name = filepath.Base(dir)
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "app"
	}
	return name
}

// A launchConfig is one configuration of a launch.json. The fields are in the order that people
// read them (encoding/json would sort a map); the ones with omitempty belong to one debugger.
type launchConfig struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Request       string   `json:"request"`
	Program       string   `json:"program"`
	Args          []string `json:"args"`
	Cwd           string   `json:"cwd"`
	PreLaunchTask string   `json:"preLaunchTask"`

	// cppdbg with gdb.
	MIMode          string            `json:"MIMode,omitempty"`
	MIDebuggerPath  string            `json:"miDebuggerPath,omitempty"`
	MIDebuggerArgs  string            `json:"miDebuggerArgs,omitempty"`
	SetupCommands   []setupCommand    `json:"setupCommands,omitempty"`
	SourceFileMap   map[string]string `json:"sourceFileMap,omitempty"`
	ExternalConsole *bool             `json:"externalConsole,omitempty"`

	// lldb-dap.
	InitCommands []string    `json:"initCommands,omitempty"`
	SourceMap    [][2]string `json:"sourceMap,omitempty"`
}

type setupCommand struct {
	Text           string `json:"text"`
	IgnoreFailures bool   `json:"ignoreFailures,omitempty"`
}

// vscodeLaunch returns the text of a launch.json for the program name, for the operating system
// goos that the program is built for and debugged on, with the debugger df.debugger (gdb through the
// C/C++ extension, or lldb through lldb-dap). gdbPath is the program that miDebuggerPath names (a
// plain name when the debugger is in PATH); maps are the -map pairs, which become sourceFileMap
// (cppdbg) or sourceMap (lldb-dap).
func vscodeLaunch(df *debugFlags, goos, name, gdbPath string) ([]byte, error) {
	program := vscodeFolder + "/bin/" + name + exeSuffixFor(goos)
	config := launchConfig{
		Type:          "cppdbg",
		Request:       "launch",
		Program:       program,
		Args:          []string{},
		Cwd:           vscodeFolder,
		PreLaunchTask: "gocpp: build",
	}
	switch df.debugger {
	case "lldb":
		config.Name = "gocpp: debug (lldb)"
		config.Type = "lldb-dap"
		config.InitCommands = []string{`command script import "` + vscodeFolder + "/.vscode/" + lldbScriptName + `"`}
		for _, m := range df.maps {
			from, to, _ := strings.Cut(m, "=")
			config.SourceMap = append(config.SourceMap, [2]string{from, to})
		}
	default:
		// gdb reads the script of the program for a safe path: the program itself, named as the
		// debugger's command line names it (slashes), in double quotes because a workspace may
		// have a space in its name. "set gocpp print-style tree" is the variables view's style.
		noWindow := false
		config.Name = "gocpp: debug"
		config.MIMode = "gdb"
		config.MIDebuggerPath = gdbPath
		config.MIDebuggerArgs = `-iex "add-auto-load-safe-path ` + program + `"`
		config.SetupCommands = []setupCommand{
			{Text: "-enable-pretty-printing", IgnoreFailures: true},
			{Text: "set gocpp print-style tree"},
		}
		config.ExternalConsole = &noWindow
		if len(df.maps) > 0 {
			config.SourceFileMap = make(map[string]string)
			for _, m := range df.maps {
				from, to, _ := strings.Cut(m, "=")
				config.SourceFileMap[from] = to
			}
		}
	}
	doc := struct {
		Version        string         `json:"version"`
		Configurations []launchConfig `json:"configurations"`
	}{"0.2.0", []launchConfig{config}}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// printVSCode is gocpp debug -vscode: it prints the launch.json for the debugger and the target of
// the build flags (-os), and for lldb writes the summaries to .vscode/gocpp_lldb.py of the current
// folder, which the configuration loads. It builds and starts nothing.
func printVSCode(f *buildFlags, df *debugFlags, args []string, stdout, stderr io.Writer) int {
	goos, _, err := f.cross.resolve()
	if err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	dir, err := getwd()
	if err != nil {
		fmt.Fprintf(stderr, "gocpp debug: %v\n", err)
		return exitErrors
	}
	if df.debugger == "" {
		df.debugger = "gdb"
	}
	hostOS := df.hostOS
	if hostOS == "" {
		hostOS = runtime.GOOS
	}
	// miDebuggerPath: the gdb that this machine has when the program is debugged here (a Windows
	// install is rarely in PATH of VS Code), else the name.
	gdbPath := "gdb"
	if goos == "windows" {
		gdbPath = "gdb.exe"
	}
	if goos == hostOS {
		if p, _, ok := findTool("gdb"); ok {
			gdbPath = debuggerPath(hostOS, p)
		}
	}
	out, err := vscodeLaunch(df, goos, vscodeProgram(args, dir), gdbPath)
	if err != nil {
		fmt.Fprintf(stderr, "gocpp debug: %v\n", err)
		return exitErrors
	}
	if df.debugger == "lldb" {
		script := filepath.Join(dir, ".vscode", lldbScriptName)
		err := os.MkdirAll(filepath.Dir(script), 0o755)
		if err == nil {
			err = os.WriteFile(script, rtfiles.LLDBScript(), 0o644)
		}
		if err != nil {
			fmt.Fprintf(stderr, "gocpp debug: %v\n", err)
			return exitErrors
		}
		fmt.Fprintf(stderr, "gocpp debug: wrote %s (the launch configuration loads it)\n", script)
	}
	stdout.Write(out)
	return 0
}
