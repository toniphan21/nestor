package nestor

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	randv2 "math/rand/v2"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const DefaultSandboxIDLength = 5
const DefaultAgentIDLength = 10
const DefaultIDLetters = "abcdefghijklmnopqrstuvwxyz"
const maxBase = 40

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func DefaultTemplate() Template {
	template := Template{
		SandboxTag:       "nestor-[name]",
		SandboxID:        fmt.Sprintf("%d:%s", DefaultSandboxIDLength, DefaultIDLetters),
		WorktreeID:       "[base]-[hash]",
		InitialBranch:    "nestor/initial-branch-[sandbox]-[hash]",
		SandboxContainer: "nestor-sandbox-[id]",
		AgentSuffix:      fmt.Sprintf("%d:%s", DefaultAgentIDLength, DefaultIDLetters),
		AgentID:          "[alias]-[id]",
	}

	_ = LoadBuiltinAgentAliases(&template)
	return template
}

func LoadBuiltinAgentAliases(template *Template) error {
	f, err := builtin.Open("assets/agent-names.txt")
	if err != nil {
		return err
	}
	defer f.Close()

	return template.LoadAgentAliases(f)
}

type Template struct {
	SandboxTag       string
	SandboxID        string
	WorktreeID       string
	InitialBranch    string
	SandboxContainer string
	AgentSuffix      string
	AgentID          string
	picker           *agentAliasPicker
}

func (t *Template) MakeSandboxID(exists []string) (string, error) {
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
	return "", fmt.Errorf("nestor: Template.MakeSandboxID no free id after 100 attempts")
}

func (t *Template) genSandboxID() string {
	return t.genRand(t.SandboxID, DefaultSandboxIDLength)
}

func (t *Template) genRand(template string, defaultLen int) string {
	parts := strings.Split(template, ":")
	if len(parts) != 2 {
		return t.rand(defaultLen, DefaultIDLetters)
	}

	length, err := strconv.Atoi(parts[0])
	if err != nil || length <= 0 {
		return t.rand(defaultLen, DefaultIDLetters)
	}

	if strings.TrimSpace(parts[1]) == "" {
		parts[1] = DefaultIDLetters
	}
	return t.rand(length, parts[1])
}

func (t *Template) MakeSandboxTag(specName string) string {
	return t.fillTemplate(t.SandboxTag, map[string]string{
		"[sandbox-name]": specName,
		"[sandboxName]":  specName,
		"$sandboxName":   specName,
		"[name]":         specName,
		"$name":          specName,
	})
}

func (t *Template) MakeWorktreeID(repository string) string {
	abs, err := filepath.Abs(repository)
	if err != nil {
		abs = filepath.Clean(repository)
	}

	hash := t.hashPath(abs)
	base := t.sanitize(filepath.Base(abs))
	if base == "" {
		return hash
	}
	return t.fillTemplate(t.WorktreeID, map[string]string{
		"[base]": base,
		"$base":  base,
		"[hash]": hash,
		"$hash":  hash,
		"[id]":   hash,
		"$id":    hash,
	})
}

func (t *Template) MakeInitialBranch(sandboxID string, dir string) string {
	hash := t.hashPath(dir)
	return t.fillTemplate(t.InitialBranch, map[string]string{
		"[sandbox]": sandboxID,
		"$sandbox":  sandboxID,
		"[id]":      sandboxID,
		"$id":       sandboxID,
		"[hash]":    hash,
		"$hash":     hash,
	})
}

func (t *Template) MakeSandboxContainer(sandboxID string) string {
	return t.fillTemplate(t.SandboxContainer, map[string]string{
		"[id]": sandboxID,
		"$id":  sandboxID,
	})
}

func (t *Template) MakeAgentID() string {
	id := t.genRand(t.AgentSuffix, DefaultSandboxIDLength)
	alias := t.picker.pick()
	return t.fillTemplate(t.AgentID, map[string]string{
		"[id]":    id,
		"$id":     id,
		"[alias]": alias,
		"$alias":  alias,
	})
}

func (t *Template) fillTemplate(template string, vars map[string]string) string {
	var out = template
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

func (t *Template) LoadAgentAliases(txt io.Reader) error {
	var names []string
	sc := bufio.NewScanner(txt)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			names = append(names, l)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	if len(names) == 0 {
		return errors.New("names: empty list")
	}

	t.picker = &agentAliasPicker{names: names}
	t.picker.shuffle()
	return nil
}

func (t *Template) PickAgentAlias() string {
	return t.picker.pick()
}

type agentAliasPicker struct {
	mu    sync.Mutex
	names []string
	idx   int
}

func (p *agentAliasPicker) pick() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := p.names[p.idx]
	p.idx++
	if p.idx == len(p.names) {
		p.idx = 0
		p.shuffle()
	}
	return n
}

func (p *agentAliasPicker) shuffle() {
	randv2.Shuffle(len(p.names), func(i, j int) {
		p.names[i], p.names[j] = p.names[j], p.names[i]
	})
}
