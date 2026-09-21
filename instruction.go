package nestor

import (
	"net/url"
	"strings"

	"nhatp.com/go/nestor/infra/fs"
)

type instruction interface {
	content() string

	saveTo(path string) error
}

type fileInstruction struct {
	path string
}

func (f *fileInstruction) content() string {
	b, err := fs.AtomicReadFile(f.path)
	if err != nil {
		return ""
	}
	return string(b)
}

func (f *fileInstruction) saveTo(path string) error {
	return fs.CopyFile(f.path, path)
}

type literalInstruction struct {
	value string
}

func (l *literalInstruction) content() string {
	return l.value
}

func (l *literalInstruction) saveTo(path string) error {
	return fs.WriteFile(path, []byte(l.value))
}

func parseInstruction(s string, kind OSKind) instruction {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "file" {
		return &literalInstruction{value: s}
	}

	p, ok := parseFilePath(s, kind)
	if !ok {
		return &literalInstruction{value: s}
	}
	return &fileInstruction{path: p}
}

func parseFilePath(s string, kind OSKind) (string, bool) {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	if u.Opaque != "" || u.Path == "" {
		return "", false
	}

	p := u.Path
	remote := u.Host != "" && u.Host != "localhost"

	if kind == OSWindows {
		switch {
		case remote:
			p = "//" + u.Host + p // UNC: \\server\share\...
		case len(p) >= 3 && p[0] == '/' && p[2] == ':':
			p = p[1:] // "/C:/x" -> "C:/x"
		}
		return strings.ReplaceAll(p, "/", `\`), true
	}

	if remote {
		return "", false // a host means another machine; no local path
	}
	return p, true // already slash-separated
}
