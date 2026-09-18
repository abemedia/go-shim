package shim

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cacheEntry returns the build cache entry and cached executable of exe if go run or go tool built
// it. The go command only checks a cached executable against the size in its entry, so replacing
// the executable and recording the new size makes it start the release binary directly.
func cacheEntry(ctx context.Context, exe string) (entry, cached string) {
	dir := filepath.Dir(exe)
	id, inCache := strings.CutSuffix(filepath.Base(dir), "-d")
	built := filepath.Base(dir) == "exe" && strings.HasPrefix(filepath.Base(filepath.Dir(filepath.Dir(dir))), "go-build")
	if !built && (!inCache || len(id) != 64) {
		return "", ""
	}

	root, err := exec.CommandContext(ctx, "go", "env", "GOCACHE").Output()
	if err != nil {
		return "", ""
	}
	buildID, err := exec.CommandContext(ctx, "go", "tool", "buildid", exe).Output()
	if err != nil {
		return "", ""
	}
	action, _, _ := strings.Cut(string(buildID), "/")
	key, err := base64.RawURLEncoding.DecodeString(action)
	if err != nil || len(key) == 0 {
		return "", ""
	}

	cache := strings.TrimSpace(string(root))
	entries, _ := filepath.Glob(filepath.Join(cache, hex.EncodeToString(key[:1]), hex.EncodeToString(key)+"*-a"))
	if len(entries) != 1 {
		return "", ""
	}
	data, err := os.ReadFile(entries[0])
	f := strings.Fields(string(data))
	if err != nil || len(f) != 5 || len(f[2]) < 2 {
		return "", ""
	}
	return entries[0], filepath.Join(cache, f[2][:2], f[2]+"-d", filepath.Base(exe))
}

func updateEntry(entry, exe string) error {
	info, err := os.Stat(exe)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(entry)
	if err != nil {
		return err
	}
	f := strings.Fields(string(data))
	if len(f) != 5 {
		return errors.New("invalid build cache entry")
	}
	return os.WriteFile(entry, fmt.Appendf(nil, "%s %s %s %20d %20s\n", f[0], f[1], f[2], info.Size(), f[4]), 0o666)
}
