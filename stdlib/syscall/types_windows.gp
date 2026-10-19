// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syscall

const (
	// STANDARD_RIGHTS_* of Go's security_windows.go.
	STANDARD_RIGHTS_REQUIRED = 0xf0000
	STANDARD_RIGHTS_READ     = 0x20000
	STANDARD_RIGHTS_WRITE    = 0x20000
	STANDARD_RIGHTS_EXECUTE  = 0x20000
	STANDARD_RIGHTS_ALL      = 0x1F0000
)

// Invented values to support what package os expects.
const (
	S_IFMT   = 0x1f000
	S_IFIFO  = 0x1000
	S_IFCHR  = 0x2000
	S_IFDIR  = 0x4000
	S_IFBLK  = 0x6000
	S_IFREG  = 0x8000
	S_IFLNK  = 0xa000
	S_IFSOCK = 0xc000
	S_ISUID  = 0x800
	S_ISGID  = 0x400
	S_ISVTX  = 0x200
	S_IRUSR  = 0x100
	S_IWRITE = 0x80
	S_IWUSR  = 0x80
	S_IXUSR  = 0x40
)

const (
	FILE_TYPE_CHAR    = 0x0002
	FILE_TYPE_DISK    = 0x0001
	FILE_TYPE_PIPE    = 0x0003
	FILE_TYPE_REMOTE  = 0x8000
	FILE_TYPE_UNKNOWN = 0x0000
)

const (
	// Windows errors.
	ERROR_FILE_NOT_FOUND      Errno = 2
	ERROR_PATH_NOT_FOUND      Errno = 3
	ERROR_ACCESS_DENIED       Errno = 5
	ERROR_NO_MORE_FILES       Errno = 18
	ERROR_HANDLE_EOF          Errno = 38
	ERROR_NETNAME_DELETED     Errno = 64
	ERROR_FILE_EXISTS         Errno = 80
	ERROR_BROKEN_PIPE         Errno = 109
	ERROR_BUFFER_OVERFLOW     Errno = 111
	ERROR_INSUFFICIENT_BUFFER Errno = 122
	ERROR_MOD_NOT_FOUND       Errno = 126
	ERROR_PROC_NOT_FOUND      Errno = 127
	ERROR_DIR_NOT_EMPTY       Errno = 145
	ERROR_ALREADY_EXISTS      Errno = 183
	ERROR_ENVVAR_NOT_FOUND    Errno = 203
	ERROR_MORE_DATA           Errno = 234
	ERROR_OPERATION_ABORTED   Errno = 995
	ERROR_IO_PENDING          Errno = 997
	ERROR_NOT_FOUND           Errno = 1168
	ERROR_PRIVILEGE_NOT_HELD  Errno = 1314
	WSAEACCES                 Errno = 10013
	WSAENOPROTOOPT            Errno = 10042
	WSAECONNABORTED           Errno = 10053
	WSAECONNRESET             Errno = 10054
)

const (
	// Invented values to support what package os expects.
	O_RDONLY       = 0x00000
	O_WRONLY       = 0x00001
	O_RDWR         = 0x00002
	O_CREAT        = 0x00040
	O_EXCL         = 0x00080
	O_NOCTTY       = 0x00100
	O_TRUNC        = 0x00200
	O_NONBLOCK     = 0x00800
	O_APPEND       = 0x00400
	O_SYNC         = 0x01000
	O_ASYNC        = 0x02000
	O_CLOEXEC      = 0x80000
	o_DIRECTORY    = 0x100000   // used by internal/syscall/windows
	o_NOFOLLOW_ANY = 0x20000000 // used by internal/syscall/windows
	o_OPEN_REPARSE = 0x40000000 // used by internal/syscall/windows
	o_WRITE_ATTRS  = 0x80000000 // used by internal/syscall/windows
)

const (
	// More invented values for signals
	SIGHUP  = Signal(0x1)
	SIGINT  = Signal(0x2)
	SIGQUIT = Signal(0x3)
	SIGILL  = Signal(0x4)
	SIGTRAP = Signal(0x5)
	SIGABRT = Signal(0x6)
	SIGBUS  = Signal(0x7)
	SIGFPE  = Signal(0x8)
	SIGKILL = Signal(0x9)
	SIGSEGV = Signal(0xb)
	SIGPIPE = Signal(0xd)
	SIGALRM = Signal(0xe)
	SIGTERM = Signal(0xf)
)

var signals = [...]string{
	1:  "hangup",
	2:  "interrupt",
	3:  "quit",
	4:  "illegal instruction",
	5:  "trace/breakpoint trap",
	6:  "aborted",
	7:  "bus error",
	8:  "floating point exception",
	9:  "killed",
	10: "user defined signal 1",
	11: "segmentation fault",
	12: "user defined signal 2",
	13: "broken pipe",
	14: "alarm clock",
	15: "terminated",
}

const (
	GENERIC_READ    = 0x80000000
	GENERIC_WRITE   = 0x40000000
	GENERIC_EXECUTE = 0x20000000
	GENERIC_ALL     = 0x10000000

	FILE_LIST_DIRECTORY   = 0x00000001
	FILE_APPEND_DATA      = 0x00000004
	_FILE_WRITE_EA        = 0x00000010
	FILE_WRITE_ATTRIBUTES = 0x00000100

	FILE_SHARE_READ              = 0x00000001
	FILE_SHARE_WRITE             = 0x00000002
	FILE_SHARE_DELETE            = 0x00000004
	FILE_ATTRIBUTE_READONLY      = 0x00000001
	FILE_ATTRIBUTE_HIDDEN        = 0x00000002
	FILE_ATTRIBUTE_SYSTEM        = 0x00000004
	FILE_ATTRIBUTE_DIRECTORY     = 0x00000010
	FILE_ATTRIBUTE_ARCHIVE       = 0x00000020
	FILE_ATTRIBUTE_NORMAL        = 0x00000080
	FILE_ATTRIBUTE_REPARSE_POINT = 0x00000400

	INVALID_FILE_ATTRIBUTES = 0xffffffff

	CREATE_NEW        = 1
	CREATE_ALWAYS     = 2
	OPEN_EXISTING     = 3
	OPEN_ALWAYS       = 4
	TRUNCATE_EXISTING = 5

	FILE_FLAG_OPEN_REPARSE_POINT = 0x00200000
	FILE_FLAG_BACKUP_SEMANTICS   = 0x02000000
	FILE_FLAG_OVERLAPPED         = 0x40000000

	HANDLE_FLAG_INHERIT    = 0x00000001
	STARTF_USESTDHANDLES   = 0x00000100
	STARTF_USESHOWWINDOW   = 0x00000001
	DUPLICATE_CLOSE_SOURCE = 0x00000001
	DUPLICATE_SAME_ACCESS  = 0x00000002

	STD_INPUT_HANDLE  = -10
	STD_OUTPUT_HANDLE = -11
	STD_ERROR_HANDLE  = -12

	FILE_BEGIN   = 0
	FILE_CURRENT = 1
	FILE_END     = 2

	LANG_ENGLISH       = 0x09
	SUBLANG_ENGLISH_US = 0x01

	FORMAT_MESSAGE_ALLOCATE_BUFFER = 256
	FORMAT_MESSAGE_IGNORE_INSERTS  = 512
	FORMAT_MESSAGE_FROM_STRING     = 1024
	FORMAT_MESSAGE_FROM_HMODULE    = 2048
	FORMAT_MESSAGE_FROM_SYSTEM     = 4096
	FORMAT_MESSAGE_ARGUMENT_ARRAY  = 8192
	FORMAT_MESSAGE_MAX_WIDTH_MASK  = 255

	MAX_PATH      = 260
	MAX_LONG_PATH = 32768

	MAX_COMPUTERNAME_LENGTH = 15

	TIME_ZONE_ID_UNKNOWN  = 0
	TIME_ZONE_ID_STANDARD = 1

	TIME_ZONE_ID_DAYLIGHT = 2
	IGNORE                = 0
	INFINITE              = 0xffffffff

	WAIT_TIMEOUT   = 258
	WAIT_ABANDONED = 0x00000080
	WAIT_OBJECT_0  = 0x00000000
	WAIT_FAILED    = 0xFFFFFFFF

	CREATE_NEW_PROCESS_GROUP   = 0x00000200
	CREATE_UNICODE_ENVIRONMENT = 0x00000400

	PROCESS_TERMINATE         = 1
	PROCESS_QUERY_INFORMATION = 0x00000400
	SYNCHRONIZE               = 0x00100000

	PAGE_READONLY          = 0x02
	PAGE_READWRITE         = 0x04
	PAGE_WRITECOPY         = 0x08
	PAGE_EXECUTE_READ      = 0x20
	PAGE_EXECUTE_READWRITE = 0x40
	PAGE_EXECUTE_WRITECOPY = 0x80

	FILE_MAP_COPY    = 0x01
	FILE_MAP_WRITE   = 0x02
	FILE_MAP_READ    = 0x04
	FILE_MAP_EXECUTE = 0x20

	CTRL_C_EVENT        = 0
	CTRL_BREAK_EVENT    = 1
	CTRL_CLOSE_EVENT    = 2
	CTRL_LOGOFF_EVENT   = 5
	CTRL_SHUTDOWN_EVENT = 6
)

const (
	// flags for CreateToolhelp32Snapshot
	TH32CS_SNAPHEAPLIST = 0x01
	TH32CS_SNAPPROCESS  = 0x02
	TH32CS_SNAPTHREAD   = 0x04
	TH32CS_SNAPMODULE   = 0x08
	TH32CS_SNAPMODULE32 = 0x10
	TH32CS_SNAPALL      = TH32CS_SNAPHEAPLIST | TH32CS_SNAPMODULE | TH32CS_SNAPPROCESS | TH32CS_SNAPTHREAD
	TH32CS_INHERIT      = 0x80000000
)

const (
	// do not reorder
	FILE_NOTIFY_CHANGE_FILE_NAME = 1 << iota
	FILE_NOTIFY_CHANGE_DIR_NAME
	FILE_NOTIFY_CHANGE_ATTRIBUTES
	FILE_NOTIFY_CHANGE_SIZE
	FILE_NOTIFY_CHANGE_LAST_WRITE
	FILE_NOTIFY_CHANGE_LAST_ACCESS
	FILE_NOTIFY_CHANGE_CREATION
)

const (
	// do not reorder
	FILE_ACTION_ADDED = iota + 1
	FILE_ACTION_REMOVED
	FILE_ACTION_MODIFIED
	FILE_ACTION_RENAMED_OLD_NAME
	FILE_ACTION_RENAMED_NEW_NAME
)

const (
	// wincrypt.h
	PROV_RSA_FULL                    = 1
	PROV_RSA_SIG                     = 2
	PROV_DSS                         = 3
	PROV_FORTEZZA                    = 4
	PROV_MS_EXCHANGE                 = 5
	PROV_SSL                         = 6
	PROV_RSA_SCHANNEL                = 12
	PROV_DSS_DH                      = 13
	PROV_EC_ECDSA_SIG                = 14
	PROV_EC_ECNRA_SIG                = 15
	PROV_EC_ECDSA_FULL               = 16
	PROV_EC_ECNRA_FULL               = 17
	PROV_DH_SCHANNEL                 = 18
	PROV_SPYRUS_LYNKS                = 20
	PROV_RNG                         = 21
	PROV_INTEL_SEC                   = 22
	PROV_REPLACE_OWF                 = 23
	PROV_RSA_AES                     = 24
	CRYPT_VERIFYCONTEXT              = 0xF0000000
	CRYPT_NEWKEYSET                  = 0x00000008
	CRYPT_DELETEKEYSET               = 0x00000010
	CRYPT_MACHINE_KEYSET             = 0x00000020
	CRYPT_SILENT                     = 0x00000040
	CRYPT_DEFAULT_CONTAINER_OPTIONAL = 0x00000080

	USAGE_MATCH_TYPE_AND = 0
	USAGE_MATCH_TYPE_OR  = 1

	X509_ASN_ENCODING   = 0x00000001
	PKCS_7_ASN_ENCODING = 0x00010000

	CERT_STORE_PROV_MEMORY = 2

	CERT_STORE_ADD_ALWAYS = 4

	CERT_STORE_DEFER_CLOSE_UNTIL_LAST_FREE_FLAG = 0x00000004

	CERT_TRUST_NO_ERROR                          = 0x00000000
	CERT_TRUST_IS_NOT_TIME_VALID                 = 0x00000001
	CERT_TRUST_IS_REVOKED                        = 0x00000004
	CERT_TRUST_IS_NOT_SIGNATURE_VALID            = 0x00000008
	CERT_TRUST_IS_NOT_VALID_FOR_USAGE            = 0x00000010
	CERT_TRUST_IS_UNTRUSTED_ROOT                 = 0x00000020
	CERT_TRUST_REVOCATION_STATUS_UNKNOWN         = 0x00000040
	CERT_TRUST_IS_CYCLIC                         = 0x00000080
	CERT_TRUST_INVALID_EXTENSION                 = 0x00000100
	CERT_TRUST_INVALID_POLICY_CONSTRAINTS        = 0x00000200
	CERT_TRUST_INVALID_BASIC_CONSTRAINTS         = 0x00000400
	CERT_TRUST_INVALID_NAME_CONSTRAINTS          = 0x00000800
	CERT_TRUST_HAS_NOT_SUPPORTED_NAME_CONSTRAINT = 0x00001000
	CERT_TRUST_HAS_NOT_DEFINED_NAME_CONSTRAINT   = 0x00002000
	CERT_TRUST_HAS_NOT_PERMITTED_NAME_CONSTRAINT = 0x00004000
	CERT_TRUST_HAS_EXCLUDED_NAME_CONSTRAINT      = 0x00008000
	CERT_TRUST_IS_OFFLINE_REVOCATION             = 0x01000000
	CERT_TRUST_NO_ISSUANCE_CHAIN_POLICY          = 0x02000000
	CERT_TRUST_IS_EXPLICIT_DISTRUST              = 0x04000000
	CERT_TRUST_HAS_NOT_SUPPORTED_CRITICAL_EXT    = 0x08000000

	CERT_CHAIN_POLICY_BASE              = 1
	CERT_CHAIN_POLICY_AUTHENTICODE      = 2
	CERT_CHAIN_POLICY_AUTHENTICODE_TS   = 3
	CERT_CHAIN_POLICY_SSL               = 4
	CERT_CHAIN_POLICY_BASIC_CONSTRAINTS = 5
	CERT_CHAIN_POLICY_NT_AUTH           = 6
	CERT_CHAIN_POLICY_MICROSOFT_ROOT    = 7
	CERT_CHAIN_POLICY_EV                = 8

	CERT_E_EXPIRED       = 0x800B0101
	CERT_E_ROLE          = 0x800B0103
	CERT_E_PURPOSE       = 0x800B0106
	CERT_E_UNTRUSTEDROOT = 0x800B0109
	CERT_E_CN_NO_MATCH   = 0x800B010F

	AUTHTYPE_CLIENT = 1
	AUTHTYPE_SERVER = 2
)

var (
	OID_PKIX_KP_SERVER_AUTH = []byte("1.3.6.1.5.5.7.3.1\x00")
	OID_SERVER_GATED_CRYPTO = []byte("1.3.6.1.4.1.311.10.3.3\x00")
	OID_SGC_NETSCAPE        = []byte("2.16.840.1.113730.4.1\x00")
)

// Pointer represents a pointer to an arbitrary Windows type.
//
// Pointer-typed fields may point to one of many different types. It's
// up to the caller to provide a pointer to the appropriate type, cast
// to Pointer. The caller must obey the unsafe.Pointer rules while
// doing so.
type Pointer *struct{}

// Invented values to support what package os expects.
type Timeval struct {
	Sec  int32
	Usec int32
}

func (tv *Timeval) Nanoseconds() int64 {
	return (int64(tv.Sec)*1e6 + int64(tv.Usec)) * 1e3
}

func NsecToTimeval(nsec int64) (tv Timeval) {
	tv.Sec = int32(nsec / 1e9)
	tv.Usec = int32(nsec % 1e9 / 1e3)
	return
}

type SecurityAttributes struct {
	Length             uint32
	SecurityDescriptor uintptr
	InheritHandle      uint32
}

type Overlapped struct {
	Internal     uintptr
	InternalHigh uintptr
	Offset       uint32
	OffsetHigh   uint32
	HEvent       Handle
}

type FileNotifyInformation struct {
	NextEntryOffset uint32
	Action          uint32
	FileNameLength  uint32
	FileName        uint16
}

type Filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// Nanoseconds returns Filetime ft in nanoseconds
// since Epoch (00:00:00 UTC, January 1, 1970).
func (ft *Filetime) Nanoseconds() int64 {
	// 100-nanosecond intervals since January 1, 1601
	nsec := int64(ft.HighDateTime)<<32 + int64(ft.LowDateTime)
	// change starting time to the Epoch (00:00:00 UTC, January 1, 1970)
	nsec -= 116444736000000000
	// convert into nanoseconds
	nsec *= 100
	return nsec
}

func NsecToFiletime(nsec int64) (ft Filetime) {
	// convert into 100-nanosecond
	nsec /= 100
	// change starting time to January 1, 1601
	nsec += 116444736000000000
	// split into high / low
	ft.LowDateTime = uint32(nsec & 0xffffffff)
	ft.HighDateTime = uint32(nsec >> 32 & 0xffffffff)
	return ft
}

type Win32finddata struct {
	FileAttributes    uint32
	CreationTime      Filetime
	LastAccessTime    Filetime
	LastWriteTime     Filetime
	FileSizeHigh      uint32
	FileSizeLow       uint32
	Reserved0         uint32
	Reserved1         uint32
	FileName          [MAX_PATH - 1]uint16
	AlternateFileName [13]uint16
}

// This is the actual system call structure.
// Win32finddata is what we committed to in Go 1.
type win32finddata1 struct {
	FileAttributes    uint32
	CreationTime      Filetime
	LastAccessTime    Filetime
	LastWriteTime     Filetime
	FileSizeHigh      uint32
	FileSizeLow       uint32
	Reserved0         uint32
	Reserved1         uint32
	FileName          [MAX_PATH]uint16
	AlternateFileName [14]uint16

	// The Microsoft documentation for this struct¹ describes three additional
	// fields: dwFileType, dwCreatorType, and wFinderFlags. However, those fields
	// are empirically only present in the macOS port of the Win32 API,² and thus
	// not needed for binaries built for Windows.
	//
	// ¹ https://docs.microsoft.com/en-us/windows/win32/api/minwinbase/ns-minwinbase-win32_find_dataw
	// ² https://golang.org/issue/42637#issuecomment-760715755
}

func copyFindData(dst *Win32finddata, src *win32finddata1) {
	dst.FileAttributes = src.FileAttributes
	dst.CreationTime = src.CreationTime
	dst.LastAccessTime = src.LastAccessTime
	dst.LastWriteTime = src.LastWriteTime
	dst.FileSizeHigh = src.FileSizeHigh
	dst.FileSizeLow = src.FileSizeLow
	dst.Reserved0 = src.Reserved0
	dst.Reserved1 = src.Reserved1

	// The src is 1 element bigger than dst, but it must be NUL.
	copy(dst.FileName[:], src.FileName[:])
	copy(dst.AlternateFileName[:], src.AlternateFileName[:])
}

type ByHandleFileInformation struct {
	FileAttributes     uint32
	CreationTime       Filetime
	LastAccessTime     Filetime
	LastWriteTime      Filetime
	VolumeSerialNumber uint32
	FileSizeHigh       uint32
	FileSizeLow        uint32
	NumberOfLinks      uint32
	FileIndexHigh      uint32
	FileIndexLow       uint32
}

const (
	GetFileExInfoStandard = 0
	GetFileExMaxInfoLevel = 1
)

type Win32FileAttributeData struct {
	FileAttributes uint32
	CreationTime   Filetime
	LastAccessTime Filetime
	LastWriteTime  Filetime
	FileSizeHigh   uint32
	FileSizeLow    uint32
}

// ShowWindow constants
const (
	// winuser.h
	SW_HIDE            = 0
	SW_NORMAL          = 1
	SW_SHOWNORMAL      = 1
	SW_SHOWMINIMIZED   = 2
	SW_SHOWMAXIMIZED   = 3
	SW_MAXIMIZE        = 3
	SW_SHOWNOACTIVATE  = 4
	SW_SHOW            = 5
	SW_MINIMIZE        = 6
	SW_SHOWMINNOACTIVE = 7
	SW_SHOWNA          = 8
	SW_RESTORE         = 9
	SW_SHOWDEFAULT     = 10
	SW_FORCEMINIMIZE   = 11
)

type StartupInfo struct {
	Cb            uint32
	_             *uint16
	Desktop       *uint16
	Title         *uint16
	X             uint32
	Y             uint32
	XSize         uint32
	YSize         uint32
	XCountChars   uint32
	YCountChars   uint32
	FillAttribute uint32
	Flags         uint32
	ShowWindow    uint16
	_             uint16
	_             *byte
	StdInput      Handle
	StdOutput     Handle
	StdErr        Handle
}

type ProcessInformation struct {
	Process   Handle
	Thread    Handle
	ProcessId uint32
	ThreadId  uint32
}

type ProcessEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [MAX_PATH]uint16
}

type Systemtime struct {
	Year         uint16
	Month        uint16
	DayOfWeek    uint16
	Day          uint16
	Hour         uint16
	Minute       uint16
	Second       uint16
	Milliseconds uint16
}

type Timezoneinformation struct {
	Bias         int32
	StandardName [32]uint16
	StandardDate Systemtime
	StandardBias int32
	DaylightName [32]uint16
	DaylightDate Systemtime
	DaylightBias int32
}

// Socket related.

const (
	AF_UNSPEC  = 0
	AF_UNIX    = 1
	AF_INET    = 2
	AF_INET6   = 23
	AF_NETBIOS = 17

	SOCK_STREAM    = 1
	SOCK_DGRAM     = 2
	SOCK_RAW       = 3
	SOCK_SEQPACKET = 5

	IPPROTO_IP   = 0
	IPPROTO_IPV6 = 0x29
	IPPROTO_TCP  = 6
	IPPROTO_UDP  = 17

	SOL_SOCKET                = 0xffff
	SO_REUSEADDR              = 4
	SO_KEEPALIVE              = 8
	SO_DONTROUTE              = 16
	SO_BROADCAST              = 32
	SO_LINGER                 = 128
	SO_RCVBUF                 = 0x1002
	SO_SNDBUF                 = 0x1001
	SO_UPDATE_ACCEPT_CONTEXT  = 0x700b
	SO_UPDATE_CONNECT_CONTEXT = 0x7010

	IOC_OUT                            = 0x40000000
	IOC_IN                             = 0x80000000
	IOC_VENDOR                         = 0x18000000
	IOC_INOUT                          = IOC_IN | IOC_OUT
	IOC_WS2                            = 0x08000000
	SIO_GET_EXTENSION_FUNCTION_POINTER = IOC_INOUT | IOC_WS2 | 6
	SIO_KEEPALIVE_VALS                 = IOC_IN | IOC_VENDOR | 4
	SIO_UDP_CONNRESET                  = IOC_IN | IOC_VENDOR | 12

	// cf. https://learn.microsoft.com/en-US/troubleshoot/windows/win32/header-library-requirement-socket-ipproto-ip

	IP_TOS             = 0x3
	IP_TTL             = 0x4
	IP_MULTICAST_IF    = 0x9
	IP_MULTICAST_TTL   = 0xa
	IP_MULTICAST_LOOP  = 0xb
	IP_ADD_MEMBERSHIP  = 0xc
	IP_DROP_MEMBERSHIP = 0xd

	IPV6_V6ONLY         = 0x1b
	IPV6_UNICAST_HOPS   = 0x4
	IPV6_MULTICAST_IF   = 0x9
	IPV6_MULTICAST_HOPS = 0xa
	IPV6_MULTICAST_LOOP = 0xb
	IPV6_JOIN_GROUP     = 0xc
	IPV6_LEAVE_GROUP    = 0xd

	SOMAXCONN = 0x7fffffff

	TCP_NODELAY = 1

	SHUT_RD   = 0
	SHUT_WR   = 1
	SHUT_RDWR = 2

	WSADESCRIPTION_LEN = 256
	WSASYS_STATUS_LEN  = 128
)

type symbolicLinkReparseBuffer struct {
	SubstituteNameOffset uint16
	SubstituteNameLength uint16
	PrintNameOffset      uint16
	PrintNameLength      uint16
	Flags                uint32
	PathBuffer           [1]uint16
}

type mountPointReparseBuffer struct {
	SubstituteNameOffset uint16
	SubstituteNameLength uint16
	PrintNameOffset      uint16
	PrintNameLength      uint16
	PathBuffer           [1]uint16
}

type reparseDataBuffer struct {
	ReparseTag        uint32
	ReparseDataLength uint16
	Reserved          uint16

	// GenericReparseBuffer
	reparseBuffer byte
}

const (
	FSCTL_GET_REPARSE_POINT          = 0x900A8
	MAXIMUM_REPARSE_DATA_BUFFER_SIZE = 16 * 1024
	_IO_REPARSE_TAG_MOUNT_POINT      = 0xA0000003
	IO_REPARSE_TAG_SYMLINK           = 0xA000000C
	SYMBOLIC_LINK_FLAG_DIRECTORY     = 0x1
	_SYMLINK_FLAG_RELATIVE           = 1
)
