package nx

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
