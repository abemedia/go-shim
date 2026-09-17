// Package shim distributes binaries built in any compiled language through go install.
//
// go install is a first-class install channel for Go developers, but it only builds Go. A shim is
// a small main package committed to the release repository of a project written in something
// else. Installed with go install, it takes the version it was installed at, downloads the
// matching release asset for the running platform, caches it, and hands over to it.
//
// A whole shim is usually this:
//
//	package main
//
//	import "github.com/abemedia/go-shim"
//
//	func main() {
//		shim.Run(shim.Config{
//			URL:     "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}",
//			Targets: shim.Rust,
//		})
//	}
//
// Everything else follows from the module path and the version go install stamped in, so a
// project tagged v1.2.3 at github.com/owner/tool fetches from its GitHub release page and
// installs a command named tool.
package shim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

// Config describes where a project's release assets live and how they are named.
//
// URL and Binary are [text/template] templates over these fields:
//
//	.Name      the command, "tool"
//	.Repo      the repository, "https://github.com/owner/tool"
//	.Tag       the release, "v1.2.3"
//	.Version   the release without its leading v, "1.2.3"
//	.Target    one entry from Targets, "x86_64-apple-darwin"
//	.Ext       ".zip" on Windows, ".tar.gz" elsewhere
//	.Exe       ".exe" on Windows, empty elsewhere
//	.GOOS      "darwin"
//	.GOARCH    "amd64"
//	.GO386     "sse2" on 386, empty elsewhere
//	.GOAMD64   "v1" on amd64, empty elsewhere
//	.GOARM     "7" on arm, empty elsewhere
//	.GOARM64   "v8.0" on arm64, empty elsewhere
//	.GOMIPS    "hardfloat" on mips and mipsle, empty elsewhere
//	.GOMIPS64  "hardfloat" on mips64 and mips64le, empty elsewhere
//	.GOPPC64   "power8" on ppc64 and ppc64le, empty elsewhere
//	.GORISCV64 "rva20u64" on riscv64, empty elsewhere
//
// Both .Tag and .Version exist because projects disagree about the leading v in a file name. Use
// whichever yours carries.
type Config struct {
	// URL is the release asset. A bare file name is resolved against the release page of the
	// module path, which is recognised for GitHub, GitLab, Bitbucket and the Gitea family:
	//
	//	URL: "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}"
	//
	// Anywhere else, give the whole address:
	//
	//	URL: "https://downloads.example.com/{{.Tag}}/{{.Name}}-{{.Target}}{{.Ext}}"
	//
	// Required.
	URL string

	// Targets lists the builds a project publishes and what each needs to run. The shim keeps the
	// entries its host can use, in the order given, and tries each of their names in turn, so a
	// project publishing several variants resolves to whichever one it actually shipped.
	// Presets such as [Rust] and [Node] cover the common naming schemes. When Targets is nil, .Target
	// is empty and assets should be named with .GOOS and .GOARCH.
	Targets []Target

	// Binary is the path to the file inside an archive, templated like URL:
	//
	//	Binary: "{{.Name}}-{{.Version}}-{{.Target}}/{{.Name}}{{.Exe}}"
	//
	// Unset, the binary is looked for as "{{.Name}}{{.Exe}}" in each of these directories in
	// turn, and finally at the archive root:
	//
	//	{{.Name}}-{{.Target}}-v{{.Version}}   {{.Name}}-{{.Target}}
	//	{{.Name}}-{{.Target}}-{{.Version}}    {{.Name}}-{{.Version}}
	//	{{.Name}}-{{.Version}}-{{.Target}}    {{.Name}}-v{{.Version}}
	//	{{.Name}}-v{{.Version}}-{{.Target}}   {{.Name}}
	Binary string

	// Format defaults to the format the asset's name implies. Set it when the name does not say,
	// which is any binary published under a name containing a dot: that is indistinguishable
	// from an archive this package cannot read.
	Format Format

	// Version is the release to download, such as "v1.2.3". It defaults to the version go
	// install stamped in. Set it from an -ldflags variable to exercise a shim built locally,
	// where no version is stamped.
	Version string
}

// Run downloads the release if it is not already cached, runs it with this process's arguments,
// and exits with its status. It does not return.
//
// Interrupts are left to the release binary, so a tool that cleans up after itself is not cut
// short. A failure is reported to standard error and exits with status 1.
func Run(c Config) {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Fprintln(os.Stderr, "no build information")
		os.Exit(1)
	}
	r, err := resolve(c, build)
	if err == nil {
		if _, statErr := os.Stat(r.bin); statErr != nil {
			err = install(context.Background(), r)
		}
	}

	if err == nil {
		// Ignore leaks into the child across exec and does not stop Ctrl-C on Windows.
		signal.Notify(make(chan os.Signal), os.Interrupt, syscall.SIGTERM) //nolint:staticcheck

		cmd := exec.Command(r.bin, os.Args[1:]...) //nolint:noctx
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		err = cmd.Run()

		var exit *exec.ExitError
		switch {
		case errors.As(err, &exit):
			os.Exit(exit.ExitCode())
		case err == nil:
			os.Exit(0)
		case errors.Is(err, syscall.ENOEXEC):
			// Otherwise one bad download would be permanent.
			os.Remove(r.bin)
			err = fmt.Errorf("%s is not executable on %s/%s; discarded", r.Tag, runtime.GOOS, runtime.GOARCH)
		}
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func install(ctx context.Context, r *release) error {
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

	dir := filepath.Dir(r.bin)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".shim-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if err := extract(name, r.format, body, a.paths, tmp); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	// The rename is atomic, so a concurrent shim never sees a partial binary. On Windows it fails
	// when another shim has already installed and is running the binary, which is not an error.
	if err := os.Rename(tmp.Name(), r.bin); err != nil {
		if _, statErr := os.Stat(r.bin); statErr != nil {
			return err
		}
	}
	return nil
}

var client = &http.Client{
	Timeout: 5 * time.Minute, // the whole download, not a single read
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if req.URL.Scheme != "https" {
			return errors.New("not https")
		}
		return nil
	},
}

func download(ctx context.Context, r *release) (asset, io.ReadCloser, error) {
	fmt.Fprintf(os.Stderr, "%s: downloading %s\n", r.Name, r.Tag)

	for _, a := range r.assets {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url, nil)
		if err != nil {
			return asset{}, nil, err
		}
		if req.URL.Scheme != "https" {
			return asset{}, nil, fmt.Errorf("%s: not https", a.url)
		}
		res, err := client.Do(req)
		if err != nil {
			return asset{}, nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
			return a, res.Body, nil
		case http.StatusNotFound, http.StatusGone:
			res.Body.Close()
		default:
			res.Body.Close()
			return asset{}, nil, fmt.Errorf("%s: %s", a.url, res.Status)
		}
	}
	return asset{}, nil, fmt.Errorf("no %s release for %s/%s", r.Tag, runtime.GOOS, runtime.GOARCH)
}
