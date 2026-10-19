package fspath

import "strings"

// URIParts splits the absolute name p into the host and the slash-separated path of its file URI
// (`file://host/path`): `C:\a b\c` is ("", "/C:/a b/c"), `\\srv\share\d` is ("srv", "/share/d"), `/home/me` is
// ("", "/home/me"). The drive letter is in upper case. The path is not escaped: that is the URI's work.
func (f Flavor) URIParts(p string) (host, uriPath string) {
	if !f.Windows {
		return "", p
	}
	p = f.Clean(p)
	vl := f.volumeLen(p)
	vol, rest := p[:vl], f.ToSlash(p[vl:])
	switch {
	case vl == 2:
		if rest == "" {
			rest = "/"
		}
		return "", "/" + strings.ToUpper(vol) + rest
	case vl > 2: // \\host\share
		parts := strings.SplitN(vol[2:], `\`, 2)
		return parts[0], "/" + parts[1] + rest
	}
	return "", rest // a rooted name with no drive: the caller adds the drive of the process
}

// FromURIParts is the opposite of URIParts. The path has been unescaped. An authority other than "" and
// "localhost" is a UNC name on Windows (`file://srv/share/d` is `\\srv\share\d`); on other flavors the host is
// kept as the first element of the name, as gopls does for `file://wsl%24/Ubuntu/x`.
func (f Flavor) FromURIParts(host, uriPath string) string {
	if !f.Windows {
		if host == "localhost" {
			host = ""
		}
		if host != "" {
			return "/" + host + uriPath
		}
		return uriPath
	}
	if host == "localhost" && len(uriPath) >= 3 && uriPath[0] == '/' && isLetter(uriPath[1]) && uriPath[2] == ':' {
		host = "" // file://localhost/C:/x is this machine's drive (RFC 8089); file://localhost/c$/x is a share
	}
	if host != "" {
		return f.Clean(`\\` + host + f.FromSlash(uriPath))
	}
	// file:///C:/x, file:///c:/x
	if len(uriPath) >= 3 && uriPath[0] == '/' && isLetter(uriPath[1]) && uriPath[2] == ':' {
		rest := uriPath[3:]
		p := strings.ToUpper(uriPath[1:2]) + ":" + f.FromSlash(rest)
		if rest == "" {
			p += `\`
		}
		return p
	}
	// file:////srv/share/x: an empty authority and two more slashes, which some tools write for a UNC name.
	if strings.HasPrefix(uriPath, "//") && len(uriPath) > 2 && uriPath[2] != '/' {
		return f.Clean(f.FromSlash(uriPath))
	}
	return f.FromSlash(uriPath)
}
