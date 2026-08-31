package nestor

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultSandboxIDLength = 5
const DefaultSandboxIDLetters = "abcdefghijklmnopqrstuvwxyz"
const maxBase = 40

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func DefaultTemplate() Template {
	return Template{
		SandboxTag:       "nestor-[name]",
		SandboxID:        fmt.Sprintf("%d:%s", DefaultSandboxIDLength, DefaultSandboxIDLetters),
		WorktreeID:       "[base]-[hash]",
		InitialBranch:    "nestor/initial-branch-[sandbox]-[hash]",
		SandboxContainer: "nestor-sandbox-[id]",
	}
}

type Template struct {
	SandboxTag       string
	SandboxID        string
	WorktreeID       string
	InitialBranch    string
	SandboxContainer string
}

func (t *Template) makeSandboxID(exists []string) (string, error) {
	taken := make(map[string]struct{}, len(exists))
	for _, e := range exists {
		taken[e] = struct{}{}
	}

	for i := 0; i < 100; i++ {
		id := t.genSandboxID()
		if _, ok := taken[id]; !ok {
			return id, nil
		}
	}
	return "", fmt.Errorf("nestor: Template.makeSandboxID no free id after 100 attempts")
}

func (t *Template) genSandboxID() string {
	parts := strings.Split(t.SandboxID, ":")
	if len(parts) != 2 {
		return t.rand(DefaultSandboxIDLength, DefaultSandboxIDLetters)
	}

	length, err := strconv.Atoi(parts[0])
	if err != nil || length <= 0 {
		return t.rand(DefaultSandboxIDLength, DefaultSandboxIDLetters)
	}

	if strings.TrimSpace(parts[1]) == "" {
		parts[1] = DefaultSandboxIDLetters
	}
	return t.rand(length, parts[1])
}

func (t *Template) makeSandboxTag(spec *SandboxSpec) string {
	var vars = map[string]string{
		"[sandbox-name]": spec.Name,
		"[sandboxName]":  spec.Name,
		"$sandboxName":   spec.Name,
		"[name]":         spec.Name,
		"$name":          spec.Name,
	}

	var out = t.SandboxTag
	for k, v := range vars {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func (t *Template) makeWorktreeID(repository string) string {
	abs, err := filepath.Abs(repository)
	if err != nil {
		abs = filepath.Clean(repository)
	}

	hash := t.hashPath(abs)
	base := t.sanitize(filepath.Base(abs))
	if base == "" {
		return hash
	}
	var vars = map[string]string{
		"[base]": base,
		"$base":  base,
		"[hash]": hash,
		"$hash":  hash,
		"[id]":   hash,
		"$id":    hash,
	}

	var out = t.WorktreeID
	for k, v := range vars {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func (t *Template) makeInitialBranch(sandboxID string, dir string) string {
	hash := t.hashPath(dir)
	var vars = map[string]string{
		"[sandbox]": sandboxID,
		"$sandbox":  sandboxID,
		"[id]":      sandboxID,
		"$id":       sandboxID,
		"[hash]":    hash,
		"$hash":     hash,
	}

	var out = t.InitialBranch
	for k, v := range vars {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func (t *Template) makeSandboxContainer(s *Sandbox) string {
	var vars = map[string]string{
		"[id]": s.ID,
		"$id":  s.ID,
	}

	var out = t.SandboxContainer
	for k, v := range vars {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func (t *Template) rand(n int, letters string) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	l := len(letters)
	for i := range b {
		b[i] = letters[int(b[i])%l]
	}
	return string(b)
}

func (t *Template) hashPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	sum := sha256.Sum256([]byte(abs))
	return strings.ToLower(b32.EncodeToString(sum[:5])) // 8 chars, 40 bits
}

func (t *Template) sanitize(s string) string {
	s = strings.ToLower(s)

	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	out := strings.Trim(b.String(), "-._")
	if len(out) > maxBase {
		out = strings.TrimRight(out[:maxBase], "-._")
	}
	return out
}
