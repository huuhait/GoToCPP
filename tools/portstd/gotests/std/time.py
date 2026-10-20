# Go 1.25.14's tests of time; see convert_std.py.
FILES = ['all']
TESTDATA = True
skip_file(
    'tzdata_test.go',
    'its only test, TestEmbeddedTZData, needs package time/tzdata (the zone database embedded in '
    'the program), which gocpp does not have; zoneinfo files come from the system',
)
for name in ['TestTimeGob', 'TestInvalidTimeGob', 'TestNotGobEncodableTime']:
    drop('time_test.go', name, 'it encodes a Time with encoding/gob, which gocpp does not have')
patch(
    'export_test.go',
    'var ParseRFC3339 = parseRFC3339[string]',
    'var ParseRFC3339 = parseRFC3339',
    'the port of parseRFC3339 takes a string and is not generic (stdlib/time/PORT.md)',
)
patch(
    'internal_test.go',
    '''	t := newTimer(runtimeNano(), 1<<63-1, empty, nil, nil)
	defer t.Stop()
''',
    '''	c := make(chan Time, 1)
	startChan(c, runtimeNano(), 1<<63-1, sendTime)
	defer func() { stopTimer(c) }()
''',
    'the port has no newTimer: a timer is a channel with a record, armed by startChan '
    '(stdlib/time/PORT.md)',
)
skip_file(
    'linkname_test.go',
    'it calls time.Time.abs, absClock and absDate by //go:linkname, the legacy hooks for '
    'third-party packages that the port left out (stdlib/time/PORT.md); gocpp has no linkname to '
    'Go code',
)
patch(
    'internal_test.go',
    '''	sources := []string{"../../lib/time/zoneinfo.zip"}
''',
    '''	sources := origPlatformZoneSources
''',
    'gocpp has no GOROOT/lib/time/zoneinfo.zip: the test zone comes from the zone database of the '
    'machine (the sources of LoadLocation, tests/README.md zoneinfo gate)',
)
ZONES = (
    'gocpp has no GOROOT/lib/time/zoneinfo.zip, the hermetic zone database that Go keeps the '
    'system sources away for'
)
patch(
    'internal_test.go',
    '''	platformZoneSources = nil
	return func() {''',
    '''	// (Go empties the sources here so that LoadLocation reads GOROOT/lib/time/zoneinfo.zip.
	// gocpp has no such file: the tests that disable the platform sources read the zone
	// database of the machine instead.)
	return func() {''',
    ZONES,
)
patch(
    'sleep_test.go',
    '''//go:linkname haveHighResSleep runtime.haveHighResSleep
var haveHighResSleep bool''',
    '''var haveHighResSleep = true''',
    "the variable is the runtime's (linkname); gocpp sleeps with the operating system's timer, "
    'which has a resolution of about a millisecond or better',
)
ALLOC = (
    'it counts allocations (testing.AllocsPerRun or runtime.MemStats), and gocpp allocates '
    'differently than Go (it counts C++ operator new); counting rule of the Phase 09 README, 2.8'
)
expect('TestUnmarshalTextAllocations', 'divergence', ALLOC)
expect(
    'TestCountMallocs',
    'divergence',
    ALLOC + ' (the test also skips itself unless GOMAXPROCS is 1)',
)
expect(
    'TestChan',
    'unsupported',
    'its subtests asynctimerchan=1 and asynctimerchan=2 test the GODEBUG that restores the timer '
    'channels of Go 1.22 (buffered, stale values after Stop); the port has the semantics of Go '
    '1.23 only (stdlib/time/PORT.md); asynctimerchan=0 passes',
)
expect(
    'TestLoadLocationFromTZData',
    'unsupported',
    'it compares LoadLocationFromTZData with the LoadLocation of the same zone from '
    'GOROOT/lib/time/zoneinfo.zip and skips itself without a GOROOT; a gocpp program has none',
)
