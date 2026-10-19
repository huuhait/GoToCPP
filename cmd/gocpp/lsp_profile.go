package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"syscall"

	lsptrace "github.com/huuhait/gocpp/internal/lsp/trace"
)

// lspProfile is what "gocpp lsp -profile dir", "-exec-trace" and "-trace
// file" write (Phase 11 step 11.12): the CPU profile (cpu.pprof) and the
// heap profile (heap.pprof) of the server in dir, the Go execution trace
// (exec.trace) when asked, and the spans of the requests as a Chrome trace.
// They are written when the server ends, or when it gets SIGINT or SIGTERM
// (then the process exits with status 130 or 143, after writing them).
type lspProfile struct {
	dir       string
	execTrace bool
	traceFile string
	rec       *lsptrace.Recorder
	stderr    io.Writer

	cpu, exec *os.File
	once      sync.Once
	sigs      chan os.Signal
}

// startLSPProfile starts what the flags ask for; nil when they ask for
// nothing.
func startLSPProfile(dir string, execTrace bool, traceFile string, stderr io.Writer) (*lspProfile, error) {
	if dir == "" && !execTrace && traceFile == "" {
		return nil, nil
	}
	if execTrace && dir == "" {
		return nil, fmt.Errorf("-exec-trace needs -profile dir")
	}
	p := &lspProfile{dir: dir, execTrace: execTrace, traceFile: traceFile, stderr: stderr}
	if traceFile != "" {
		p.rec = lsptrace.NewRecorder(0)
	}
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		f, err := os.Create(filepath.Join(dir, "cpu.pprof"))
		if err != nil {
			return nil, err
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return nil, err
		}
		p.cpu = f
		if execTrace {
			f, err := os.Create(filepath.Join(dir, "exec.trace"))
			if err != nil {
				p.stop()
				return nil, err
			}
			if err := trace.Start(f); err != nil {
				f.Close()
				p.stop()
				return nil, err
			}
			p.exec = f
		}
	}
	p.sigs = make(chan os.Signal, 1)
	signal.Notify(p.sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig, ok := <-p.sigs
		if !ok {
			return
		}
		p.stop()
		code := 130
		if sig == syscall.SIGTERM {
			code = 143
		}
		os.Exit(code)
	}()
	return p, nil
}

// stop writes the profiles and the trace. It is safe to call twice.
func (p *lspProfile) stop() {
	p.once.Do(func() {
		if p.sigs != nil {
			signal.Stop(p.sigs)
		}
		if p.cpu != nil {
			pprof.StopCPUProfile()
			p.report(p.cpu.Close())
		}
		if p.exec != nil {
			trace.Stop()
			p.report(p.exec.Close())
		}
		if p.dir != "" {
			runtime.GC() // the heap profile shows what is live now
			f, err := os.Create(filepath.Join(p.dir, "heap.pprof"))
			if err == nil {
				err = pprof.Lookup("heap").WriteTo(f, 0)
				if cerr := f.Close(); err == nil {
					err = cerr
				}
			}
			p.report(err)
		}
		if p.traceFile != "" {
			f, err := os.Create(p.traceFile)
			if err == nil {
				err = p.rec.WriteChrome(f)
				if cerr := f.Close(); err == nil {
					err = cerr
				}
			}
			p.report(err)
		}
	})
}

func (p *lspProfile) report(err error) {
	if err != nil {
		fmt.Fprintf(p.stderr, "gocpp lsp: %v\n", err)
	}
}
