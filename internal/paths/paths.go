package paths

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var schemeRegexp = regexp.MustCompile(`^.+://.*`)

func IsURI(ref string) bool {
	return schemeRegexp.MatchString(ref)
}

func IsDir(p string) (bool, error) {
	fileInfo, err := os.Stat(p)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), nil
}

func FilePathToURI(p string) (string, error) {
	var err error
	if !filepath.IsAbs(p) {
		p, err = filepath.Abs(p)
		if err != nil {
			return "", err
		}
	}

	if runtime.GOOS == "windows" {
		if strings.HasPrefix(p, `\\`) {
			return "file://" + filepath.ToSlash(strings.TrimPrefix(p, `\\`)), nil
		}
		return "file:///" + filepath.ToSlash(p), nil
	}
	return "file://" + p, nil
}

// examples:
//
// - unix file: file://laptop/some%20dir/file.tgz
//
// - windows drive: file:///C:/Documents%20and%20Settings/file.tgz
//
// - windows share: file://laptop/My%20Documents/file.tgz
//
func URIToFilePath(uri string) (string, error) {
	var (
		osPath string
		err    error
	)

	osPath = filepath.FromSlash(strings.TrimPrefix(uri, "file://"))

	if osPath, err = url.PathUnescape(osPath); err != nil {
		return "", nil
	}

	if runtime.GOOS == "windows" {
		if strings.HasPrefix(osPath, `\`) {
			return strings.TrimPrefix(osPath, `\`), nil
		}
		return `\\` + osPath, nil
	}
	return osPath, nil
}

func ToAbsolute(uri, relativeTo string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		if !filepath.IsAbs(parsed.Path) {
			absPath := filepath.Join(relativeTo, parsed.Path)
			return FilePathToURI(absPath)
		}
	}

	return uri, nil
}

func FilterReservedNames(p string) string {
	// The following keys are reserved on Windows
	// https://docs.microsoft.com/en-us/windows/win32/fileio/naming-a-file?redirectedfrom=MSDN#win32-file-namespaces
	reservedNameConversions := map[string]string{
		"aux": "a_u_x",
		"com": "c_o_m",
		"con": "c_o_n",
		"lpt": "l_p_t",
		"nul": "n_u_l",
		"prn": "p_r_n",
	}
	for k, v := range reservedNameConversions {
		p = strings.Replace(p, k, v, -1)
	}

	return p
}

//WindowsDir is equivalent to path.Dir or filepath.Dir but always for Windows paths
//reproduced because Windows implementation is not exported
func WindowsDir(p string) string {
	pathElements := strings.Split(p, `\`)
	if len(pathElements) < 1 {
		return ""
	}

	dirName := strings.Join(pathElements[:len(pathElements)-1], `\`)

	return dirName
}

//WindowsBasename is equivalent to path.Basename or filepath.Basename but always for Windows paths
//reproduced because Windows implementation is not exported
func WindowsBasename(p string) string {
	pathElements := strings.Split(p, `\`)
	if len(pathElements) < 1 {
		return ""
	}

	return pathElements[len(pathElements)-1]
}

//WindowsToSlash is equivalent to path.Basename or filepath.Basename but always for Windows paths
//reproduced because Windows implementation is not exported
func WindowsToSlash(p string) string {
	return strings.ReplaceAll(p, `\`, "/")[2:] // strip volume, convert slashes
}

//WindowsPathSID returns the appropriate SID for a given UID and GID
func WindowsPathSID(uid, gid int) string {
	if uid == 0 && gid == 0 {
		return "S-1-5-32-544" // BUILTIN\Administrators
	}
	return "S-1-5-32-545" // BUILTIN\Users
}
