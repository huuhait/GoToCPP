// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

// The functions of this file are written in C++ (runtime/include/gocpp/testing.hpp), because the
// packages that Go's testing imports for them (sync, time, os, runtime) are declaration stubs
// until later steps of Phase 09. Their names are the Go names of what they replace:
// libLock and libUnlock are sync.Mutex, libNow is time.Now, libFrames is runtime.Callers with
// runtime.CallersFrames, libLookupEnv, libSetEnv and libUnsetEnv are os.LookupEnv, Setenv and
// Unsetenv. See ADR-056.

// libLock and libUnlock take and release the one recursive lock that guards the state of every
// test and benchmark. User code never runs while it is held, and nothing waits for a goroutine
// under it.
//
//gocpp:native gp::testing::lock
func libLock()

//gocpp:native gp::testing::unlock
func libUnlock()

// libNow returns a monotonic clock in nanoseconds.
//
//gocpp:native gp::testing::now
func libNow() int64

// libArgs returns the arguments of the program without its name.
//
//gocpp:native gp::testing::args
func libArgs() []string

// libProgramName is the name the program was started with (os.Args[0]).
//
//gocpp:native gp::testing::program_name
func libProgramName() string

// libNCPU is runtime.GOMAXPROCS(0).
//
//gocpp:native gp::testing::ncpu
func libNCPU() int

//gocpp:native gp::testing::goos
func libGOOS() string

//gocpp:native gp::testing::goarch
func libGOARCH() string

// libCPUName is the model name of the processor, "" if it is not known (sysinfo.CPUName).
//
//gocpp:native gp::testing::cpu_name
func libCPUName() string

// libFrames describes the stack of the calling goroutine, innermost frame first, one line
// "function<TAB>file<TAB>line" for each frame.
//
//gocpp:native gp::testing::frames
func libFrames(max int) string

// libCaptureStart redirects the standard output to a temporary file and returns an error text ("" when
// it worked), libCaptureStop restores it and returns what was written.
//
//gocpp:native gp::testing::capture_start
func libCaptureStart() string

//gocpp:native gp::testing::capture_stop
func libCaptureStop() string

//gocpp:native gp::testing::lookup_env
func libLookupEnv(key string) (string, bool)

//gocpp:native gp::testing::set_env
func libSetEnv(key, value string) string

//gocpp:native gp::testing::unset_env
func libUnsetEnv(key string) string

//gocpp:native gp::testing::make_temp_dir
func libMakeTempDir(prefix string) (dir string, err string)

//gocpp:native gp::testing::make_dir
func libMakeDir(path string) string

//gocpp:native gp::testing::remove_all
func libRemoveAll(path string) string

// libExit ends the process, like os.Exit.
//
//gocpp:native gp::testing::exit
func libExit(code int)

// libGoexit is runtime.Goexit.
//
//gocpp:native gp::goexit
func libGoexit()

// libAlarmWait waits for ns nanoseconds and reports true, or reports false at once when
// libAlarmCancel was called (or ns <= 0): the timer of -test.timeout.
//
//gocpp:native gp::testing::alarm_wait
func libAlarmWait(ns int64) bool

//gocpp:native gp::testing::alarm_cancel
func libAlarmCancel()

// The list of the tests that run, for the message of a timeout.
//
//gocpp:native gp::testing::running_add
func libRunningAdd(name string, start int64)

//gocpp:native gp::testing::running_delete
func libRunningDelete(name string)

//gocpp:native gp::testing::running_list
func libRunningList() string

// libGetwd and libChdir are os.Getwd and os.Chdir; the errors are texts, "" when it worked.
//
//gocpp:native gp::testing::getwd
func libGetwd() (dir string, err string)

//gocpp:native gp::testing::chdir
func libChdir(dir string) string

// libReadFiles returns the names of the files (not folders) in the folder dir, sorted, one per
// line, and "" for a folder that does not exist; an error is its text. libReadFile returns the
// content of a file.
//
//gocpp:native gp::testing::read_files
func libReadFiles(dir string) (names string, err string)

//gocpp:native gp::testing::read_file
func libReadFile(path string) (data string, err string)

// libParseFloat parses a floating-point literal (strconv.ParseFloat).
//
//gocpp:native gp::testing::parse_float
func libParseFloat(s string) (float64, bool)

// libSleep sleeps for ns nanoseconds.
//
//gocpp:native gp::testing::sleep_ns
func libSleep(ns int64)

// libAllocCounting(1) turns on the counting of allocations, libAllocCounting(-1) takes the request
// back; libAllocs returns the number of allocations and of bytes counted so far. They count the
// calls of C++ operator new, which are not the allocations of Go's compiler (testing.cpp).
//
//gocpp:native gp::testing::alloc_counting
func libAllocCounting(delta int)

//gocpp:native gp::testing::alloc_stats
func libAllocs() (n, bytes int)
