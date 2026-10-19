//go:build !windows

package winpath

func privateFolder(string, []string) (string, error) { return "", ErrPrivateUnsupported }

func verifyPrivate(string) error { return ErrPrivateUnsupported }
