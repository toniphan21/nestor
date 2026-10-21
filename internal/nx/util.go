package nx

import "strings"

func Dedup[T comparable](list []T) []T {
	seen := make(map[T]bool)
	var out []T
	for _, v := range list {
		if _, have := seen[v]; have {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func ReplacePathPrefix(path, prefix, replacement string) string {
	if prefix == "" {
		return path
	}
	if rel, ok := strings.CutPrefix(path, prefix); ok {
		return replacement + rel
	}
	return path
}

func CutHome(path, uhome string) string {
	return ReplacePathPrefix(path, uhome, "~")
}
