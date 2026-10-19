//go:build !noselfupdate

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
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

	newVersion, err := replaceBinary(context.Background(), target, binary, 0o755)
	if err != nil {
		return err
	}

	fmt.Printf("updated %s v%s -> v%s\n", target, nestor.Version, newVersion)
	return nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

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
	defer func() {
		_ = gz.Close()
	}()

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

func replaceBinary(ctx context.Context, path string, data []byte, perm os.FileMode) (version string, err error) {
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close() // harmless if already closed
			_ = os.Remove(tmp)
		}
	}()

	if _, err = f.Write(data); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Chmod(tmp, perm); err != nil {
		return "", err
	}

	if version, err = probeVersion(ctx, tmp); err != nil {
		return "", fmt.Errorf("verify new binary: %w", err)
	}

	if err = os.Rename(tmp, path); err != nil {
		return "", err
	}
	return version, nil
}

func probeVersion(ctx context.Context, bin string) (string, error) {
	abs, err := filepath.Abs(bin)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, abs, "version").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%w: %s", err, bytes.TrimSpace(ee.Stderr))
		}
		return "", err
	}

	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", errors.New("empty version output")
	}
	return v, nil
}
