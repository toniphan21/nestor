//go:build !noselfupdate

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
)

const upgradeCommand = "nestor self-update"

func selfUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: shortDesc["self-update"],
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := cmd.Flags().GetString("url")
			if err != nil {
				return err
			}
			return selfUpdate(url)
		},
	}
	cmd.Flags().String("url", "", "directory to download the release from (defaults to the latest GitHub release)")
	return cmd
}

func selfUpdate(baseURL string) error {
	if baseURL == "" {
		if _, hasUpdate := nestor.CheckUpdate(""); !hasUpdate {
			fmt.Printf("nestor is already up to date (%s)\n", nestor.Version)
			return nil
		}
		baseURL = nestor.DefaultDownloadURL
	}

	client := &http.Client{Timeout: 60 * time.Second}
	asset := fmt.Sprintf("%s_%s_%s.tar.gz", nestor.BinaryName, runtime.GOOS, runtime.GOARCH)

	archive, err := fetch(client, baseURL+"/"+asset)
	if err != nil {
		return err
	}
	sums, err := fetch(client, baseURL+"/checksums.txt")
	if err != nil {
		return err
	}
	if err := verifyChecksum(archive, sums, asset); err != nil {
		return err
	}

	binary, err := extractBinary(archive, nestor.BinaryName)
	if err != nil {
		return err
	}

	target, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}

	if err := fs.AtomicWriteFile(target, binary, 0o755); err != nil {
		return err
	}

	fmt.Printf("updated %s\n", target)
	return nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func verifyChecksum(archive, sums []byte, name string) error {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			sum := sha256.Sum256(archive)
			got := hex.EncodeToString(sum[:])
			if got != fields[0] {
				return fmt.Errorf("checksum mismatch for %s: want %s, got %s", name, fields[0], got)
			}
			return nil
		}
	}
	return fmt.Errorf("no checksum entry found for %s", name)
}

func extractBinary(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == name {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}
