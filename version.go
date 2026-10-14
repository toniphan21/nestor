package nestor

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	BinaryName = "nestor"
	Version    = "dev"
	GoOS       = "darwin"
	GoArch     = "amd64"
)

const (
	LatestReleaseURL   = "https://api.github.com/repos/toniphan21/nestor/releases/latest"
	DefaultDownloadURL = "https://github.com/toniphan21/nestor/releases/latest/download"
)

func CheckUpdate(url string) (string, bool) {
	if url == "" {
		url = LatestReleaseURL
	}

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	if latest == "" {
		return "", false
	}
	return latest, isNewerVersion(latest, Version)
}

func isNewerVersion(a, b string) bool {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv int
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}
