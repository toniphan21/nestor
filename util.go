package nestor

import (
	"crypto/rand"
	"fmt"
)

func randID(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	l := len(letters)
	for i := range b {
		b[i] = letters[int(b[i])%l]
	}
	return string(b)
}

func makeID(n int, exists []string) (string, error) {
	taken := make(map[string]struct{}, len(exists))
	for _, e := range exists {
		taken[e] = struct{}{}
	}

	for i := 0; i < 100; i++ {
		id := randID(n)
		if _, ok := taken[id]; !ok {
			return id, nil
		}
	}
	return "", fmt.Errorf("nestor: makeID no free id of length %d after 100 attempts", n)
}
