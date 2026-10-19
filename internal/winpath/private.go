package winpath

import "errors"

// A private folder is a folder below a shared place (%ProgramData%) that only its user can use (Phase 14 step 14.2, ADR-081):
// where gocpp keeps what it needs a path for that the compilers can read when the user's own folders are not such a
// path (a user name with a character outside the ANSI code page and no 8.3 short name). %ProgramData% is shared by
// all users, and what is created there inherits "Users: read and write" entries; so the cache and the temporary
// files of one user would be readable by the others, and a hostile user could create the folder first, own it, and put objects and
// headers into the cache of the victim. PrivateFolder avoids both:
//
//   - the name has the user's SID in it (ProgramData\gocpp-paths-S-1-5-21-...), so two users never meet in a folder;
//   - a folder it creates has a DACL that is protected (no inheritance) and gives full control to the user, SYSTEM and
//     Administrators and to no one else;
//   - a folder that exists is used only if its owner is the user, SYSTEM or Administrators, it is not a link, and its
//     DACL gives no one else any access (a precreated folder, or one whose ACL was opened later, is refused);
//   - every level below the first is made and checked the same way.
//
// An error means "this place cannot be used": the caller takes its next choice or warns.

// ErrPrivateUnsupported is the error of PrivateFolder on systems other than Windows.
var ErrPrivateUnsupported = errors.New("private folders are for Windows")

// PrivateFolder returns the folder base\gocpp-paths-<SID>\parts..., made private as above.
func PrivateFolder(base string, parts ...string) (string, error) { return privateFolder(base, parts) }

// VerifyPrivate checks that the existing folder p is private for the user (the checks above), and returns why not.
func VerifyPrivate(p string) error { return verifyPrivate(p) }
