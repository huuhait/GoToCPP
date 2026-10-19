//go:build windows

package winpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	advapi32                   = syscall.NewLazyDLL("advapi32.dll")
	procConvertStringSD        = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procGetNamedSecurityInfoW  = advapi32.NewProc("GetNamedSecurityInfoW")
	procGetAclInformation      = advapi32.NewProc("GetAclInformation")
	procGetAce                 = advapi32.NewProc("GetAce")
	procEqualSid               = advapi32.NewProc("EqualSid")
	procConvertStringSidToSidW = advapi32.NewProc("ConvertStringSidToSidW")
	procConvertSidToStringSidW = advapi32.NewProc("ConvertSidToStringSidW")
	procCreateDirectoryW       = kernel32.NewProc("CreateDirectoryW")
	procLocalFree              = kernel32.NewProc("LocalFree")
)

const (
	seFileObject              = 1
	ownerSecurityInformation  = 1
	daclSecurityInformation   = 4
	aclSizeInformation        = 2
	accessAllowedACEType      = 0
	fileAttributeReparsePoint = 0x400
	sddlRevision1             = 1
	// Every access right that lets a principal read or change the folder or what is in it; an allow entry for anyone else that has
	// one of these bits makes the folder not private.
	anyAccess = 0x1 | 0x2 | 0x4 | 0x8 | 0x10 | 0x20 | 0x40 | 0x100 | 0x10000 | 0x40000 | 0x80000 | 0x10000000 | 0x20000000 | 0x40000000 | 0x80000000
)

// currentUser returns the SID of the user of this process as a string and as a SID.
func currentUser() (string, *syscall.SID, error) {
	tok, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", nil, err
	}
	defer tok.Close()
	u, err := tok.GetTokenUser()
	if err != nil {
		return "", nil, err
	}
	s, err := u.User.Sid.String()
	if err != nil {
		return "", nil, err
	}
	return s, u.User.Sid, nil
}

func sidFromString(s string) (*syscall.SID, error) {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil, err
	}
	var sid *syscall.SID
	if r, _, e := procConvertStringSidToSidW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&sid))); r == 0 {
		return nil, e
	}
	return sid, nil // (a few bytes that live as long as the process)
}

func equalSid(a, b *syscall.SID) bool {
	r, _, _ := procEqualSid.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)))
	return r != 0
}

func sidString(sid *syscall.SID) string {
	var p *uint16
	if r, _, _ := procConvertSidToStringSidW.Call(uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(&p))); r == 0 {
		return "?"
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(p)))
	return syscall.UTF16ToString((*[256]uint16)(unsafe.Pointer(p))[:])
}

// trustedSIDs returns the principals that may own a private folder and have access to it: the user, SYSTEM and Administrators.
func trustedSIDs(user *syscall.SID) ([]*syscall.SID, error) {
	out := []*syscall.SID{user}
	for _, s := range []string{"S-1-5-18", "S-1-5-32-544"} { // SYSTEM, BUILTIN\Administrators
		sid, err := sidFromString(s)
		if err != nil {
			return nil, err
		}
		out = append(out, sid)
	}
	return out, nil
}

func privateFolder(base string, parts []string) (string, error) {
	userStr, user, err := currentUser()
	if err != nil {
		return "", err
	}
	trusted, err := trustedSIDs(user)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "gocpp-paths-"+userStr)
	if err := ensurePrivate(dir, userStr, user, trusted); err != nil {
		return "", err
	}
	for _, p := range parts {
		dir = filepath.Join(dir, p)
		if err := ensurePrivate(dir, userStr, user, trusted); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func verifyPrivate(p string) error {
	_, user, err := currentUser()
	if err != nil {
		return err
	}
	trusted, err := trustedSIDs(user)
	if err != nil {
		return err
	}
	return checkPrivate(p, trusted)
}

// ensurePrivate makes the folder dir if it is not there (with the protected DACL) and checks it.
func ensurePrivate(dir, userStr string, user *syscall.SID, trusted []*syscall.SID) error {
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		if err := createProtected(dir, userStr); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return checkPrivate(dir, trusted)
}

// createProtected makes the folder with a protected DACL: full control for SYSTEM, Administrators and the user, inherited by what is
// created inside, nothing inherited from the parent.
func createProtected(dir, userStr string) error {
	sddl, err := syscall.UTF16PtrFromString("D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + userStr + ")")
	if err != nil {
		return err
	}
	var sd unsafe.Pointer
	if r, _, e := procConvertStringSD.Call(uintptr(unsafe.Pointer(sddl)), sddlRevision1, uintptr(unsafe.Pointer(&sd)), 0); r == 0 {
		return fmt.Errorf("security descriptor: %w", e)
	}
	defer procLocalFree.Call(uintptr(sd))
	sa := struct {
		Length  uint32
		Desc    uintptr
		Inherit int32
	}{Length: uint32(unsafe.Sizeof(struct {
		Length  uint32
		Desc    uintptr
		Inherit int32
	}{})), Desc: uintptr(sd)}
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	if r, _, e := procCreateDirectoryW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&sa))); r == 0 {
		if e == syscall.ERROR_ALREADY_EXISTS {
			return os.ErrExist
		}
		return &os.PathError{Op: "mkdir", Path: dir, Err: e}
	}
	return nil
}

// checkPrivate is the verification: a folder (not a link), owned by a trusted principal, whose DACL gives no one else any access.
func checkPrivate(dir string, trusted []*syscall.SID) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a plain folder", dir)
	}
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	// Lstat reports a junction (a mount point) as a folder, with no symbolic-link bit, so the attribute is what tells it: this check is not
	// redundant (TestPrivateFolderRefusesWhatIsNotPrivate/a_link fails without it). A symbolic link is caught by the mode above.
	if attrs, err := syscall.GetFileAttributes(p); err != nil || attrs&fileAttributeReparsePoint != 0 {
		return fmt.Errorf("%s is a link", dir)
	}
	var owner, dacl, sd unsafe.Pointer
	if r, _, _ := procGetNamedSecurityInfoW.Call(uintptr(unsafe.Pointer(p)), seFileObject, ownerSecurityInformation|daclSecurityInformation,
		uintptr(unsafe.Pointer(&owner)), 0, uintptr(unsafe.Pointer(&dacl)), 0, uintptr(unsafe.Pointer(&sd))); r != 0 {
		return fmt.Errorf("reading the security of %s: error %d", dir, r)
	}
	defer procLocalFree.Call(uintptr(sd))
	isTrusted := func(sid *syscall.SID) bool {
		for _, t := range trusted {
			if equalSid(sid, t) {
				return true
			}
		}
		return false
	}
	if owner == nil || !isTrusted((*syscall.SID)(owner)) {
		return fmt.Errorf("%s is owned by %s, not by you, SYSTEM or Administrators: refused", dir, sidString((*syscall.SID)(owner)))
	}
	if dacl == nil {
		return fmt.Errorf("%s has no access list (everyone has access): refused", dir)
	}
	var info struct{ AceCount, BytesInUse, BytesFree uint32 }
	if r, _, e := procGetAclInformation.Call(uintptr(dacl), uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), aclSizeInformation); r == 0 {
		return e
	}
	for i := uint32(0); i < info.AceCount; i++ {
		var ace unsafe.Pointer
		if r, _, e := procGetAce.Call(uintptr(dacl), uintptr(i), uintptr(unsafe.Pointer(&ace))); r == 0 {
			return e
		}
		typ := *(*byte)(ace)
		if typ != accessAllowedACEType {
			continue // a deny entry does no harm, and the other kinds do not occur on files
		}
		mask := *(*uint32)(unsafe.Add(ace, 4))
		sid := (*syscall.SID)(unsafe.Add(ace, 8))
		if !isTrusted(sid) && mask&anyAccess != 0 {
			return fmt.Errorf("%s gives access (%#x) to %s: refused", dir, mask, sidString(sid))
		}
	}
	return nil
}
