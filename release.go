package shim

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"text/template"
)

type release struct {
	Name    string
	Repo    string
	Tag     string
	Version string
	Target  string
	Ext     string
	Exe     string

	GOOS      string
	GOARCH    string
	GO386     string
	GOAMD64   string
	GOARM     string
	GOARM64   string
	GOMIPS    string
	GOMIPS64  string
	GOPPC64   string
	GORISCV64 string

	format Format
	bin    string
	assets []asset
}

type asset struct {
	url   string
	paths []string
}

func resolve(c Config, build *debug.BuildInfo) (*release, error) {
	repo := trimMajor(build.Main.Path)
	h := host(build)

	r := &release{
		Name:      path.Base(repo),
		Repo:      "https://" + repo,
		Tag:       c.Version,
		GOOS:      h.GOOS,
		GOARCH:    h.GOARCH,
		GO386:     h.GO386,
		GOAMD64:   h.GOAMD64,
		GOARM:     h.GOARM,
		GOARM64:   h.GOARM64,
		GOMIPS:    h.GOMIPS,
		GOMIPS64:  h.GOMIPS64,
		GOPPC64:   h.GOPPC64,
		GORISCV64: h.GORISCV64,
		Ext:       ".tar.gz",
		format:    c.Format,
	}
	if runtime.GOOS == "windows" {
		r.Ext, r.Exe = ".zip", ".exe"
	}

	if c.URL == "" {
		return nil, errors.New("Config.URL is required")
	}
	if r.Tag == "" && strings.HasPrefix(build.Main.Version, "v") {
		r.Tag = strings.TrimSuffix(build.Main.Version, "+incompatible")
	}
	if r.Tag == "" {
		return nil, fmt.Errorf("built from source; install with `go install %s@latest`", build.Main.Path)
	}
	r.Version = strings.TrimPrefix(r.Tag, "v")

	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	r.bin = filepath.Join(cache, "go-shim", repo+"@"+r.Tag, r.Name+r.Exe)

	// Every template is rendered now so that a bad one fails before any request.
	if r.assets, err = candidates(c, *r, repo, h); err != nil {
		return nil, err
	}
	return r, nil
}

func binaryPaths(configured string, r release) ([]string, error) {
	if configured != "" {
		path, err := render("Binary", configured, r)
		return []string{path}, err
	}

	name := r.Name + r.Exe
	dirs := []string{
		r.Name + "-" + r.Target + "-v" + r.Version,
		r.Name + "-" + r.Target + "-" + r.Version,
		r.Name + "-" + r.Version + "-" + r.Target,
		r.Name + "-v" + r.Version + "-" + r.Target,
		r.Name + "-" + r.Target,
		r.Name + "-" + r.Version,
		r.Name + "-v" + r.Version,
		r.Name,
	}

	paths := make([]string, 0, len(dirs)+1)
	for _, dir := range dirs {
		paths = append(paths, dir+"/"+name)
	}
	return append(paths, name), nil
}

func candidates(c Config, r release, repo string, h Target) ([]asset, error) {
	var targets []string
	if c.Targets == nil {
		targets = []string{""}
	}
	for _, t := range c.Targets {
		if t.matches(h) {
			targets = append(targets, t.Names...)
		}
	}
	if len(targets) == 0 {
		platform := h.GOOS + "/" + h.GOARCH
		if h.GOARM != "" {
			platform += " with GOARM=" + h.GOARM
		}
		return nil, fmt.Errorf("no %s release for %s", r.Tag, platform)
	}

	forge, _, _ := strings.Cut(repo, "/")
	prefix, err := render("URL", releasePage(forge), r)
	if err != nil {
		return nil, err
	}

	assets := make([]asset, 0, len(targets))
	for _, target := range targets {
		r.Target = target
		address, err := render("URL", c.URL, r)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(address, "://") {
			address = prefix + address
		}
		paths, err := binaryPaths(c.Binary, r)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset{url: address, paths: paths})
	}
	return assets, nil
}

func releasePage(host string) string {
	switch host {
	case "gitlab.com":
		return "{{.Repo}}/-/releases/{{.Tag}}/downloads/"
	case "bitbucket.org":
		// Bitbucket downloads are not scoped to a tag, so the file name carries the version.
		return "{{.Repo}}/downloads/"
	default:
		// Gitea, Forgejo and Codeberg use GitHub's layout.
		return "{{.Repo}}/releases/download/{{.Tag}}/"
	}
}

func render(field, text string, r release) (string, error) {
	tmpl, err := template.New(field).Parse(text)
	if err != nil {
		return "", fmt.Errorf("Config.%s: %w", field, err) //nolint:staticcheck
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, r); err != nil {
		return "", fmt.Errorf("Config.%s: %w", field, err) //nolint:staticcheck
	}
	return out.String(), nil
}

func trimMajor(module string) string {
	dir, last := path.Split(module)
	if dir == "" || len(last) < 2 || last[0] != 'v' {
		return module
	}
	for _, digit := range last[1:] {
		if digit < '0' || digit > '9' {
			return module
		}
	}
	return strings.TrimSuffix(dir, "/")
}
