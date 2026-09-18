package shim

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
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
	Libc      string

	format Format
	assets []asset
}

type asset struct {
	url   string
	paths []string
}

func resolve(ctx context.Context, c Config, build *debug.BuildInfo) (*release, error) {
	if c.URL == "" {
		return nil, errors.New("Config.URL is required")
	}
	tag := c.Version
	if tag == "" && strings.HasPrefix(build.Main.Version, "v") {
		tag = strings.TrimSuffix(build.Main.Version, "+incompatible")
	}
	if tag == "" {
		return nil, fmt.Errorf("built from source; install with `go install %s@latest`", build.Path)
	}
	repo, err := repository(ctx, build.Main.Path)
	if err != nil {
		return nil, err
	}

	// go install names the binary after the package, skipping a major version suffix such as /v2.
	dir, name := path.Split(build.Path)
	if dir != "" && isVersionElement(name) {
		name = path.Base(dir)
	}

	r := &release{
		Name:    name,
		Repo:    repo,
		Tag:     tag,
		Version: strings.TrimPrefix(tag, "v"),
		Ext:     ".tar.gz",
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		Libc:    hostLibc(),
		format:  c.Format,
	}
	if r.GOOS == "windows" {
		r.Ext, r.Exe = ".zip", ".exe"
	}
	for _, s := range build.Settings {
		switch s.Key {
		case "GO386":
			r.GO386 = s.Value
		case "GOAMD64":
			r.GOAMD64 = s.Value
		case "GOARM":
			r.GOARM = s.Value
		case "GOARM64":
			r.GOARM64 = s.Value
		case "GOMIPS":
			r.GOMIPS = s.Value
		case "GOMIPS64":
			r.GOMIPS64 = s.Value
		case "GOPPC64":
			r.GOPPC64 = s.Value
		case "GORISCV64":
			r.GORISCV64 = s.Value
		}
	}

	r.assets, err = candidates(c, *r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func repository(ctx context.Context, module string) (string, error) {
	// GitHub and Bitbucket serve no go-import tag below a repository's root, so the go command maps
	// their paths by rule.
	if f := strings.SplitN(module, "/", 4); len(f) >= 3 && (f[0] == "github.com" || f[0] == "bitbucket.org") {
		return "https://" + strings.Join(f[:3], "/"), nil
	}

	tag, err := goImport(ctx, module, module)
	if err != nil {
		return "", err
	}
	// Like the go command, trust a shorter prefix only if its own page declares the same tag.
	if tag.prefix != module {
		root, err := goImport(ctx, tag.prefix, module)
		if err != nil {
			return "", err
		}
		if root != tag {
			return "", fmt.Errorf("%s and %s disagree about go-import for %s", module, tag.prefix, tag.prefix)
		}
	}
	return strings.TrimSuffix(tag.repo, ".git"), nil
}

type goImportTag struct{ prefix, vcs, repo, subdir string }

func goImport(ctx context.Context, path, module string) (goImportTag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+path+"?go-get=1", nil)
	if err != nil {
		return goImportTag{}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return goImportTag{}, err
	}
	defer res.Body.Close()

	tags, err := goImportTags(res.Body)
	if len(tags) == 0 && res.StatusCode != http.StatusOK {
		return goImportTag{}, fmt.Errorf("%s: %s", req.URL, res.Status)
	}
	if err != nil {
		return goImportTag{}, err
	}
	var matches []goImportTag
	for _, tag := range tags {
		if tag.vcs != "mod" && (module == tag.prefix || strings.HasPrefix(module, tag.prefix+"/")) {
			matches = append(matches, tag)
		}
	}
	switch len(matches) {
	case 0:
		return goImportTag{}, fmt.Errorf("%s: no go-import meta tag", req.URL)
	case 1:
		return matches[0], nil
	default:
		return goImportTag{}, fmt.Errorf("%s: multiple go-import meta tags match %s", req.URL, module)
	}
}

func goImportTags(r io.Reader) ([]goImportTag, error) {
	d := xml.NewDecoder(r)
	d.Strict = false
	var tags []goImportTag
	for {
		t, err := d.RawToken()
		if err != nil {
			if errors.Is(err, io.EOF) || len(tags) > 0 {
				return tags, nil
			}
			return nil, err
		}
		if e, ok := t.(xml.StartElement); ok && strings.EqualFold(e.Name.Local, "body") {
			return tags, nil
		}
		if e, ok := t.(xml.EndElement); ok && strings.EqualFold(e.Name.Local, "head") {
			return tags, nil
		}
		e, ok := t.(xml.StartElement)
		if !ok || !strings.EqualFold(e.Name.Local, "meta") {
			continue
		}
		var name, content string
		for _, a := range e.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "name":
				name = a.Value
			case "content":
				content = a.Value
			}
		}
		if f := strings.Fields(content); name == "go-import" && (len(f) == 3 || len(f) == 4) {
			tag := goImportTag{prefix: f[0], vcs: f[1], repo: f[2]}
			if len(f) == 4 {
				tag.subdir = f[3]
			}
			tags = append(tags, tag)
		}
	}
}

func isVersionElement(s string) bool {
	if len(s) < 2 || s[0] != 'v' || s[1] == '0' || s == "v1" {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

var (
	glibcLoaders = map[string][]string{
		"386":      {"/lib/ld-linux.so.2"},
		"amd64":    {"/lib64/ld-linux-x86-64.so.2"},
		"arm":      {"/lib/ld-linux-armhf.so.3", "/lib/ld-linux.so.3"},
		"arm64":    {"/lib/ld-linux-aarch64.so.1"},
		"loong64":  {"/lib64/ld-linux-loongarch-lp64d.so.1"},
		"mips":     {"/lib/ld.so.1"},
		"mipsle":   {"/lib/ld.so.1"},
		"mips64":   {"/lib64/ld64.so.1"},
		"mips64le": {"/lib64/ld64.so.1"},
		"ppc64":    {"/lib64/ld64.so.2"},
		"ppc64le":  {"/lib64/ld64.so.2"},
		"riscv64":  {"/lib/ld-linux-riscv64-lp64d.so.1"},
		"s390x":    {"/lib/ld64.so.1"},
	}
	muslLoaders = map[string][]string{
		"386":      {"/lib/ld-musl-i386.so.1"},
		"amd64":    {"/lib/ld-musl-x86_64.so.1"},
		"arm":      {"/lib/ld-musl-armhf.so.1", "/lib/ld-musl-arm.so.1"},
		"arm64":    {"/lib/ld-musl-aarch64.so.1"},
		"loong64":  {"/lib/ld-musl-loongarch64.so.1"},
		"mips":     {"/lib/ld-musl-mips.so.1"},
		"mipsle":   {"/lib/ld-musl-mipsel.so.1"},
		"mips64":   {"/lib/ld-musl-mips64.so.1"},
		"mips64le": {"/lib/ld-musl-mips64el.so.1"},
		"ppc64":    {"/lib/ld-musl-powerpc64.so.1"},
		"ppc64le":  {"/lib/ld-musl-powerpc64le.so.1"},
		"riscv64":  {"/lib/ld-musl-riscv64.so.1"},
		"s390x":    {"/lib/ld-musl-s390x.so.1"},
	}
)

// hostLibc reports the host's C library the same way cmd/link picks an ELF interpreter.
func hostLibc() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	exists := func(name string) bool {
		_, err := os.Stat(name)
		return err == nil
	}
	if slices.ContainsFunc(glibcLoaders[runtime.GOARCH], exists) {
		return "gnu"
	}
	if slices.ContainsFunc(muslLoaders[runtime.GOARCH], exists) {
		return "musl"
	}
	return ""
}

func candidates(c Config, r release) ([]asset, error) {
	var targets []string
	if c.Targets == nil {
		targets = []string{""}
	}
	for _, t := range c.Targets {
		if matches(t, r) {
			targets = append(targets, t.Names...)
		}
	}
	if len(targets) == 0 {
		platform := r.GOOS + "/" + r.GOARCH
		if r.GOARM != "" {
			platform += " with GOARM=" + r.GOARM
		}
		return nil, fmt.Errorf("no %s release for %s", r.Tag, platform)
	}

	prefix := releasePage(r)
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

//nolint:cyclop
func matches(t Target, r release) bool {
	return (t.GOOS == "" || t.GOOS == r.GOOS) &&
		(t.GOARCH == "" || t.GOARCH == r.GOARCH) &&
		(t.Libc == "" || t.Libc == r.Libc) &&
		(t.GO386 == "" || float(t.GO386, r.GO386, "sse2")) &&
		(t.GOAMD64 == "" || atLeast(t.GOAMD64, r.GOAMD64, "v")) &&
		(t.GOARM == "" || goarm(t.GOARM, r.GOARM)) &&
		(t.GOARM64 == "" || goarm64(t.GOARM64, r.GOARM64)) &&
		(t.GOMIPS == "" || float(t.GOMIPS, r.GOMIPS, "hardfloat")) &&
		(t.GOMIPS64 == "" || float(t.GOMIPS64, r.GOMIPS64, "hardfloat")) &&
		(t.GOPPC64 == "" || atLeast(t.GOPPC64, r.GOPPC64, "power")) &&
		(t.GORISCV64 == "" || atLeast(t.GORISCV64, r.GORISCV64, "rva"))
}

func float(want, have, hard string) bool {
	return want == "softfloat" || want == hard && have == hard
}

func atLeast(want, have, prefix string) bool {
	w := number(want, prefix)
	return w >= 0 && number(have, prefix) >= w
}

func number(value, prefix string) int {
	value, ok := strings.CutPrefix(value, prefix)
	if !ok {
		return -1
	}
	end := 0
	for end < len(value) && '0' <= value[end] && value[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(value[:end])
	if err != nil {
		return -1
	}
	return n
}

func goarm(want, have string) bool {
	soft := func(v string) bool { return v == "5" || strings.HasSuffix(v, ",softfloat") }
	w := number(want, "")
	return w >= 5 && number(have, "") >= w && (soft(want) || !soft(have))
}

func goarm64(want, have string) bool {
	w, wext, _ := strings.Cut(want, ",")
	h, _, _ := strings.Cut(have, ",")
	wv, err := strconv.ParseFloat(strings.TrimPrefix(w, "v"), 64)
	hv, _ := strconv.ParseFloat(strings.TrimPrefix(h, "v"), 64)
	if hv >= 9 && wv < 9 {
		// ARMv9.0 is based on ARMv8.5, so a v9.x host only satisfies v8 requirements up to v8.(x+5).
		hv -= 0.5
	}
	if err != nil || hv < wv {
		return false
	}
	for ext := range strings.SplitSeq(wext, ",") {
		if ext != "" && !strings.Contains(have, ","+ext) && !(ext == "lse" && hv >= 8.1) {
			return false
		}
	}
	return true
}

func releasePage(r release) string {
	forge, _, _ := strings.Cut(strings.TrimPrefix(r.Repo, "https://"), "/")
	switch forge {
	case "gitlab.com":
		return r.Repo + "/-/releases/" + r.Tag + "/downloads/"
	case "bitbucket.org":
		// Bitbucket downloads are not per tag.
		return r.Repo + "/downloads/"
	default:
		// Gitea, Forgejo and Codeberg use GitHub's layout.
		return r.Repo + "/releases/download/" + r.Tag + "/"
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
