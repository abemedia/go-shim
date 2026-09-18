// Package shim distributes native binaries built in any language through go install.
//
// A shim is a main package committed to a non-Go project's repository. On its first run it
// downloads the release asset matching its module version and platform, replaces its own
// executable with the release binary, and runs it.
//
// A complete shim typically looks like this:
//
//	package main
//
//	import "github.com/abemedia/go-shim"
//
//	func main() {
//		shim.Main(shim.Config{
//			URL:     "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}",
//			Targets: shim.Rust,
//		})
//	}
package shim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
)

// Config describes where a project's release assets live and how they are named.
//
// URL and Binary are [text/template] templates rendered with these fields:
//
//	.Name      the executable name, "tool"
//	.Repo      the repository URL, "https://github.com/owner/tool"
//	.Tag       the release tag, "v1.2.3"
//	.Version   the tag without its leading v, "1.2.3"
//	.Target    a name from the matching Target, "x86_64-apple-darwin"
//	.Ext       ".zip" on Windows, ".tar.gz" elsewhere
//	.Exe       ".exe" on Windows, empty elsewhere
//	.GOOS      runtime.GOOS, "darwin"
//	.GOARCH    runtime.GOARCH, "amd64"
//	.GO386    "sse2" on 386, empty elsewhere
//	.GOAMD64   "v1" on amd64, empty elsewhere
//	.GOARM     "7" on arm, empty elsewhere
//	.GOARM64   "v8.0" on arm64, empty elsewhere
//	.GOMIPS    "hardfloat" on mips and mipsle, empty elsewhere
//	.GOMIPS64  "hardfloat" on mips64 and mips64le, empty elsewhere
//	.GOPPC64   "power8" on ppc64 and ppc64le, empty elsewhere
//	.GORISCV64 "rva20u64" on riscv64, empty elsewhere
//	.Libc      "gnu" or "musl" on Linux, empty elsewhere
type Config struct {
	// URL is the release asset's URL. A bare file name is resolved against the release download
	// URL of the module's repository, which works for GitHub, GitLab, Bitbucket and the Gitea
	// family:
	//
	//	URL: "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}"
	//
	// For any other host, give an absolute URL:
	//
	//	URL: "https://downloads.example.com/{{.Tag}}/{{.Name}}-{{.Target}}{{.Ext}}"
	//
	// Required.
	URL string

	// Targets lists the builds a project publishes and the platform each requires. The shim skips
	// entries the host cannot run and uses the first remaining name, in order, that has a published
	// asset. Presets such as [Rust] and [Node] cover common naming schemes. If Targets is nil,
	// .Target is empty.
	Targets []Target

	// Binary is the executable's path within an archive, templated like URL:
	//
	//	Binary: "{{.Name}}-{{.Version}}-{{.Target}}/{{.Name}}{{.Exe}}"
	//
	// If it is unset, the shim looks for "{{.Name}}{{.Exe}}" in each of these directories in order:
	//
	//	{{.Name}}-{{.Target}}-v{{.Version}}
	//	{{.Name}}-{{.Target}}-{{.Version}}
	//	{{.Name}}-{{.Version}}-{{.Target}}
	//	{{.Name}}-v{{.Version}}-{{.Target}}
	//	{{.Name}}-{{.Target}}
	//	{{.Name}}-{{.Version}}
	//	{{.Name}}-v{{.Version}}
	//	{{.Name}}
	//	the archive root
	Binary string

	// Format is inferred from the asset's file extension when zero.
	// Set it to [FormatBinary] for an uncompressed executable whose name contains a dot.
	Format Format

	// Version is the release tag to download, such as "v1.2.3".
	// It defaults to the installed version.
	Version string
}

// Main runs [Run] and exits with the release binary's exit status. An interrupt exits with status
// 130. Any other error is printed to standard error and exits with status 1.
func Main(c Config) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := Run(ctx, c)
	stop()

	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		os.Exit(exit.ExitCode())
	case errors.Is(err, context.Canceled):
		os.Exit(130)
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

// Run downloads the release, installs it in place of this executable, and runs it with this
// process's arguments. Under go run and go tool the release replaces the build cache entry
// instead, so later runs start it directly. ctx bounds the installation, not the release binary.
//
// On success Run does not return. On Windows it instead returns when the release binary exits,
// reporting a non-zero exit status as an [exec.ExitError].
func Run(ctx context.Context, c Config) error {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("no build information")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	r, err := resolve(ctx, c, build)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	entry, cached := cacheEntry(ctx, exe)
	if cached != "" {
		exe, dir = cached, filepath.Dir(filepath.Dir(cached))
	}
	if err := install(ctx, r, dir, exe); err != nil {
		return err
	}
	if entry != "" {
		_ = updateEntry(entry, exe)
	}

	// Windows does not implement Exec, so there the release binary runs as a child process.
	_ = syscall.Exec(exe, os.Args, os.Environ())

	cmd := exec.Command(exe, os.Args[1:]...) //nolint:noctx
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func install(ctx context.Context, r *release, dir, exe string) error {
	a, body, err := download(ctx, r)
	if err != nil {
		return err
	}
	defer body.Close()

	name := a.url
	if i := strings.IndexAny(name, "?#"); i >= 0 {
		name = name[:i]
	}
	name = path.Base(name)

	tmp, err := os.CreateTemp(dir, ".shim-*")
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("cannot write to %s: %w", dir, fs.ErrPermission)
	}
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if err := extract(name, r.format, body, a.paths, tmp); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	info, err := tmp.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s: binary is empty", name)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return replace(tmp.Name(), exe)
}

func download(ctx context.Context, r *release) (asset, io.ReadCloser, error) {
	fmt.Fprintf(os.Stderr, "%s: downloading %s\n", r.Name, r.Tag)

	var denied error
	for _, a := range r.assets {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url, nil)
		if err != nil {
			return asset{}, nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return asset{}, nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
			if t, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type")); t == "text/html" {
				res.Body.Close()
				return asset{}, nil, fmt.Errorf("%s: got an HTML page instead of a release asset", a.url)
			}
			return a, res.Body, nil
		case http.StatusNotFound, http.StatusGone:
			res.Body.Close()
		case http.StatusForbidden:
			// S3 and CloudFront answer 403 for missing objects.
			res.Body.Close()
			denied = fmt.Errorf("%s: %s", a.url, res.Status)
		default:
			res.Body.Close()
			return asset{}, nil, fmt.Errorf("%s: %s", a.url, res.Status)
		}
	}
	if denied != nil {
		return asset{}, nil, denied
	}
	return asset{}, nil, fmt.Errorf("no %s release for %s/%s", r.Tag, r.GOOS, r.GOARCH)
}

// replace moves src to exe. Windows cannot overwrite or delete a running executable, so if the
// rename fails exe is moved aside first and deleted once this process exits.
func replace(src, exe string) error {
	if os.Rename(src, exe) == nil {
		return nil
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(src, exe); err != nil {
		_ = os.Rename(old, exe)
		return err
	}
	if os.Remove(old) != nil {
		deleteAfterExit(old)
	}
	return nil
}

var client = &http.Client{Transport: httpsOnly{http.DefaultTransport}}

type httpsOnly struct{ http.RoundTripper }

func (t httpsOnly) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" {
		return nil, errors.New("not https")
	}
	return t.RoundTripper.RoundTrip(req)
}
