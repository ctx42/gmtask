package gmprj

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ctx42/gomake/pkg/gomake"
)

// ErrNoConfig is returned when project is missing configuration file.
var ErrNoConfig = errors.New("no project configuration file")

// ProjectName returns the project name derived from origin — a directory path,
// git remote, or Go import spec — as its last path element with any trailing
// ".git" removed. It returns an empty string when origin has no such element.
func ProjectName(origin string) string {
	name := filepath.Base(origin)
	if name == "." {
		return ""
	}
	name = strings.TrimSuffix(name, ".git")
	return name
}

// GoModuleName returns the Go module path derived from name — a directory path,
// git remote, or Go import spec. It strips a leading "ssh://" or "git@" and a
// trailing ".git", reduces an absolute path to its base name, and rewrites ":"
// to "/" so an "git@host:owner/repo" remote becomes "host/owner/repo".
func GoModuleName(name string) string {
	name = strings.TrimPrefix(name, "ssh://")
	name = strings.TrimPrefix(name, "git@")
	name = strings.TrimSuffix(name, ".git")
	if filepath.IsAbs(name) {
		name = ProjectName(name)
	}
	name = strings.ReplaceAll(name, ":", "/")
	return name
}

// GoPkgName returns the Go package name derived from name — a directory path,
// git remote, or Go import spec. It takes the [ProjectName] and, when that name
// is "-"-separated, keeps only its last segment (so "acme-cool-svc" yields
// "svc"). It returns an empty string when the name resolves to none.
func GoPkgName(name string) string {
	name = ProjectName(name)
	names := strings.Split(name, "-")
	if len(names) == 1 {
		return names[0]
	}
	return names[len(names)-1]
}

// Root returns absolute path to a project root directory starting at pth. It
// does it by walking up directories till it finds "configs/project.conf", if
// it's not found or an error occurred it will return empty string.
func Root(pth string, elem ...string) (string, error) {
	var err error
	pth, err = filepath.Abs(pth)
	if err != nil {
		return "", err
	}
	start := pth
	for {
		if pth == "/" {
			return "", fmt.Errorf("%w starting at %s", ErrNoConfig, start)
		}
		check := filepath.Join(pth, "configs", "project.conf")
		if gomake.FileExists(check) {
			break
		}
		pth = filepath.Dir(pth)
	}
	return filepath.Join(append([]string{pth}, elem...)...), nil
}

func toAlphabeticalEnv(m map[string]string) []string {
	// TODO(rz): test this.
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+m[key])
	}
	return env
}
