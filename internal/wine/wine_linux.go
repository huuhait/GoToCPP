//go:build linux

// Package wine runs Windows programs under Wine for gocpp's tests (Phase 06 step 6.13, ADR-055).
// Only tests import it. It follows the rules that docs job 0.6b and the dev shell found:
//
//   - Wine gets a WINEPREFIX of its own, and a HOME and XDG_* folders of its own, under the folder
//     the caller gives (a test's temporary folder): nothing lands in the user's real home (~/.wine,
//     ~/.local/share/applications, ~/.config/menus, ~/.cache/wine);
//   - WINEDLLOVERRIDES has winemenubuilder.exe=d (no desktop entries, MIME files or icons), mscoree
//     and mshtml disabled (no offer to install Mono and Gecko) and winewayland.drv=d (Wine's
//     Wayland driver otherwise finds the real desktop's socket, even without WAYLAND_DISPLAY, and
//     opens windows there while it creates the prefix);
//   - DISPLAY and WAYLAND_DISPLAY are unset, so no X or Wayland connection is tried at all;
//   - WINEDEBUG=-all keeps Wine's own messages off stderr, which the tests compare;
//   - the prefix is made once, up front (wineboot -i), with its own wineserver in the foreground
//     of a child process (wineserver -f -p: persistent, so that the cold start of a server, about 6
//     seconds, is paid once), and Close ends that server by its own PID, then every process that
//     still has the prefix in its environment.
package wine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Overrides is the WINEDLLOVERRIDES of every Wine process that the tests start.
const Overrides = "winemenubuilder.exe=d;mscoree,mshtml=;winewayland.drv=d"

// Available returns why Wine can't be used on this machine ("wine is not in PATH", "this is not
// Linux"), or "" when it can.
func Available() string {
	if _, err := exec.LookPath("wine"); err != nil {
		return "wine is not in PATH"
	}
	if _, err := exec.LookPath("wineserver"); err != nil {
		return "wineserver is not in PATH"
	}
	return ""
}

// An Env is a Wine prefix and a server for it.
type Env struct {
	dir     string
	prefix  string
	wine    string
	server  *exec.Cmd
	serverW chan struct{} // closed when the server exited
}

// New makes a Wine prefix in dir (created if needed) and starts its server. It takes several
// seconds (a cold start of Wine). dir must be a folder that only this Env uses.
func New(dir string) (*Env, error) {
	if why := Available(); why != "" {
		return nil, errors.New(why)
	}
	wine, _ := exec.LookPath("wine")
	wineserver, _ := exec.LookPath("wineserver")
	e := &Env{dir: dir, prefix: filepath.Join(dir, "prefix"), wine: wine}
	for _, d := range []string{e.prefix, e.home(), filepath.Join(e.home(), ".local", "share"), filepath.Join(e.home(), ".config"), filepath.Join(e.home(), ".cache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	// The server first, in the foreground of a child process whose PID is ours to kill.
	e.server = exec.Command(wineserver, "-f", "-p")
	e.server.Env = e.Environ()
	e.server.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if err := e.server.Start(); err != nil {
		return nil, fmt.Errorf("starting wineserver: %w", err)
	}
	e.serverW = make(chan struct{})
	go func() { e.server.Wait(); close(e.serverW) }()
	// The prefix. wineboot prints a few lines on stderr (creating a prefix does); they are the
	// reason that the prefix is made once, here, and not by the first test program.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := e.Command(ctx, "wineboot", "-i")
	// A file, not a pipe: the helper processes that wineboot starts keep its output open for as long
	// as the server runs, and a pipe would never end.
	log, err := os.Create(filepath.Join(dir, "wineboot.log"))
	if err != nil {
		e.Close()
		return nil, err
	}
	defer log.Close()
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		e.Close()
		out, _ := os.ReadFile(log.Name())
		return nil, fmt.Errorf("wineboot -i: %v\n%s", err, out)
	}
	// A first program: Wine starts its helper processes (winedevice.exe, ...) for the first
	// program that runs, and they keep that program's output open for as long as the server
	// runs. Starting it here, with a file for its output, keeps the tests' pipes closed when
	// their programs exit.
	warm := e.Command(ctx, "cmd", "/c", "exit")
	warm.Stdout, warm.Stderr = log, log
	if err := warm.Run(); err != nil {
		e.Close()
		out, _ := os.ReadFile(log.Name())
		return nil, fmt.Errorf("the first Wine program: %v\n%s", err, out)
	}
	return e, nil
}

func (e *Env) home() string { return filepath.Join(e.dir, "home") }

// Prefix returns the WINEPREFIX.
func (e *Env) Prefix() string { return e.prefix }

// Wine returns the path of the wine program.
func (e *Env) Wine() string { return e.wine }

// Environ returns the environment of a process that runs Wine programs for this Env: the
// process's own, with the Wine variables set and DISPLAY and WAYLAND_DISPLAY gone, and HOME and
// the XDG folders in the Env's own folder.
func (e *Env) Environ() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "DISPLAY", "WAYLAND_DISPLAY", "WAYLAND_SOCKET", "WINEPREFIX", "WINEDEBUG", "WINEDLLOVERRIDES", "WINEARCH",
			"HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_DIRS", "XDG_CONFIG_DIRS", "WINESERVER":
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"WINEPREFIX="+e.prefix,
		"WINEDEBUG=-all",
		"WINEDLLOVERRIDES="+Overrides,
		"HOME="+e.home(),
		"XDG_DATA_HOME="+filepath.Join(e.home(), ".local", "share"),
		"XDG_CONFIG_HOME="+filepath.Join(e.home(), ".config"),
		"XDG_CACHE_HOME="+filepath.Join(e.home(), ".cache"),
	)
}

// Command returns a command that runs "wine args..." in the Env, in a process group of its own
// that Close can find again (it also has the prefix in its environment).
func (e *Env) Command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, e.wine, args...)
	cmd.Env = e.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

// Close ends the server (by its own PID) and then every process that has this Env's prefix in
// its environment (the helper processes that Wine starts: services.exe, explorer.exe, ...),
// and waits until they are gone. It is safe to call twice.
func (e *Env) Close() {
	if e == nil || e.server == nil {
		return
	}
	if e.server.Process != nil {
		syscall.Kill(-e.server.Process.Pid, syscall.SIGTERM)
		select {
		case <-e.serverW:
		case <-time.After(10 * time.Second):
			syscall.Kill(-e.server.Process.Pid, syscall.SIGKILL)
			<-e.serverW
		}
	}
	e.server = nil
	deadline := time.Now().Add(10 * time.Second)
	for {
		pids := e.Processes()
		if len(pids) == 0 {
			return
		}
		sig := syscall.SIGTERM
		if time.Now().After(deadline.Add(-5 * time.Second)) {
			sig = syscall.SIGKILL
		}
		for _, pid := range pids {
			syscall.Kill(pid, sig)
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Processes returns the PIDs of the processes (not this one) that have this Env's prefix in
// their environment: what Close must end. Linux only (/proc); elsewhere it returns nil.
func (e *Env) Processes() []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	want := []byte("WINEPREFIX=" + e.prefix + "\x00")
	var pids []int
	for _, ent := range entries {
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		env, err := os.ReadFile("/proc/" + ent.Name() + "/environ")
		if err != nil {
			continue
		}
		if bytes.Contains(append([]byte{0}, env...), append([]byte{0}, want...)) {
			pids = append(pids, pid)
		}
	}
	return pids
}
