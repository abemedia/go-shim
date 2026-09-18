package shim

import (
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
)

func TestResolveURLs(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		build  *debug.BuildInfo
		page   string
		want   []string
	}{
		{
			name: "one candidate per target name",
			config: Config{
				URL:     "{{.Name}}-{{.Version}}-{{.Target}}.tar.gz",
				Targets: []Target{{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Names: []string{"musl", "gnu"}}},
			},
			build: &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want: []string{
				"https://github.com/owner/tool/releases/download/v1.2.3/tool-1.2.3-musl.tar.gz",
				"https://github.com/owner/tool/releases/download/v1.2.3/tool-1.2.3-gnu.tar.gz",
			},
		},
		{
			name:   "a whole address is left alone",
			config: Config{URL: "{{.Repo}}/uploads/{{.Tag}}/{{.Name}}_{{.GOOS}}_{{.GOARCH}}"},
			build:  &debug.BuildInfo{Path: "example.com/owner/tool", Main: debug.Module{Path: "example.com/owner/tool", Version: "v1"}},
			page:   `<meta name="go-import" content="example.com/owner/tool git https://example.com/owner/tool">`,
			want:   []string{"https://example.com/owner/tool/uploads/v1/tool_" + runtime.GOOS + "_" + runtime.GOARCH},
		},
		{
			name:   "Config.Version wins over the stamped version",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz", Version: "v9"},
			build:  &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://github.com/owner/tool/releases/download/v9/tool-9.tar.gz"},
		},
		{
			name:   "github",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://github.com/owner/tool/releases/download/v1.2.3/tool-1.2.3.tar.gz"},
		},
		{
			name:   "codeberg",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Path: "codeberg.org/owner/tool", Main: debug.Module{Path: "codeberg.org/owner/tool", Version: "v1.2.3"}},
			page:   `<meta name="go-import" content="codeberg.org/owner/tool git https://codeberg.org/owner/tool.git">`,
			want:   []string{"https://codeberg.org/owner/tool/releases/download/v1.2.3/tool-1.2.3.tar.gz"},
		},
		{
			name:   "gitlab",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Path: "gitlab.com/owner/tool", Main: debug.Module{Path: "gitlab.com/owner/tool", Version: "v1.2.3"}},
			page:   `<meta name="go-import" content="gitlab.com/owner/tool git https://gitlab.com/owner/tool.git">`,
			want:   []string{"https://gitlab.com/owner/tool/-/releases/v1.2.3/downloads/tool-1.2.3.tar.gz"},
		},
		{
			name:   "vanity import path",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Path: "example.com/tool", Main: debug.Module{Path: "example.com/tool", Version: "v1.2.3"}},
			page:   `<meta name="go-import" content="example.com/tool git https://github.com/owner/tool">`,
			want:   []string{"https://github.com/owner/tool/releases/download/v1.2.3/tool-1.2.3.tar.gz"},
		},
		{
			name:   "bitbucket",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Path: "bitbucket.org/owner/tool", Main: debug.Module{Path: "bitbucket.org/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://bitbucket.org/owner/tool/downloads/tool-1.2.3.tar.gz"},
		},
		{
			name: "build settings",
			config: Config{
				URL: "https://example.com/{{.GO386}}|{{.GOAMD64}}|{{.GOARM}}|{{.GOARM64}}|" +
					"{{.GOMIPS}}|{{.GOMIPS64}}|{{.GOPPC64}}|{{.GORISCV64}}",
			},
			build: &debug.BuildInfo{
				Path: "github.com/owner/tool",
				Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"},
				Settings: []debug.BuildSetting{
					{Key: "GO386", Value: "sse2"},
					{Key: "GOAMD64", Value: "v3"},
					{Key: "GOARM", Value: "7,hardfloat"},
					{Key: "GOARM64", Value: "v8.1,lse"},
					{Key: "GOMIPS", Value: "hardfloat"},
					{Key: "GOMIPS64", Value: "softfloat"},
					{Key: "GOPPC64", Value: "power9"},
					{Key: "GORISCV64", Value: "rva22u64"},
				},
			},
			want: []string{"https://example.com/sse2|v3|7,hardfloat|v8.1,lse|hardfloat|softfloat|power9|rva22u64"},
		},
		{
			name:   "an unset build setting renders as nothing",
			config: Config{URL: "https://example.com/{{with .GOARM}}v{{.}}{{end}}x"},
			build:  &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://example.com/x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func(rt http.RoundTripper) { client.Transport = rt }(client.Transport)
			client.Transport = page(tt.page)

			r, err := resolve(t.Context(), tt.config, tt.build)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			got := make([]string, len(r.assets))
			for i, a := range r.assets {
				got[i] = a.url
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("urls = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePaths(t *testing.T) {
	bin := "tool"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	tests := []struct {
		name   string
		config Config
		want   []string
	}{
		{
			name:   "conventional directories",
			config: Config{URL: "{{.Name}}{{.Ext}}"},
			want: []string{
				"tool--v1.2.3/" + bin, "tool--1.2.3/" + bin,
				"tool-1.2.3-/" + bin, "tool-v1.2.3-/" + bin,
				"tool-/" + bin, "tool-1.2.3/" + bin,
				"tool-v1.2.3/" + bin, "tool/" + bin, bin,
			},
		},
		{
			name: "named with the target",
			config: Config{
				URL:     "{{.Name}}{{.Ext}}",
				Targets: []Target{{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Names: []string{"musl"}}},
			},
			want: []string{
				"tool-musl-v1.2.3/" + bin, "tool-musl-1.2.3/" + bin,
				"tool-1.2.3-musl/" + bin, "tool-v1.2.3-musl/" + bin,
				"tool-musl/" + bin, "tool-1.2.3/" + bin,
				"tool-v1.2.3/" + bin, "tool/" + bin, bin,
			},
		},
		{
			name:   "configured",
			config: Config{URL: "{{.Name}}{{.Ext}}", Binary: "other"},
			want:   []string{"other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build := &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}}
			r, err := resolve(t.Context(), tt.config, build)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if !slices.Equal(r.assets[0].paths, tt.want) {
				t.Errorf("paths = %q, want %q", r.assets[0].paths, tt.want)
			}
		})
	}
}

func TestResolveErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		build   *debug.BuildInfo
		page    string
		wantErr string
	}{
		{
			name:    "missing url",
			config:  Config{},
			build:   &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "Config.URL",
		},
		{
			name:    "unparsable url",
			config:  Config{URL: "{{"},
			build:   &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "Config.URL",
		},
		{
			name:    "unknown field",
			config:  Config{URL: "{{.Nope}}"},
			build:   &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "Config.URL",
		},
		{
			name:    "unknown platform",
			config:  Config{URL: "a", Targets: []Target{{GOOS: "plan9", GOARCH: "386", Names: []string{"x"}}}},
			build:   &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "no v1.2.3 release for " + runtime.GOOS,
		},
		{
			name:    "built from source",
			config:  Config{URL: "a"},
			build:   &debug.BuildInfo{Path: "github.com/owner/tool", Main: debug.Module{Path: "github.com/owner/tool", Version: "(devel)"}},
			wantErr: "built from source",
		},
		{
			name:    "no go-import tag",
			config:  Config{URL: "a"},
			build:   &debug.BuildInfo{Path: "example.com/tool", Main: debug.Module{Path: "example.com/tool", Version: "v1.2.3"}},
			page:    `<meta name="go-import" content="example.com/other git https://github.com/owner/other">`,
			wantErr: "no go-import meta tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func(rt http.RoundTripper) { client.Transport = rt }(client.Transport)
			client.Transport = page(tt.page)

			_, err := resolve(t.Context(), tt.config, tt.build)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("resolve error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestResolveModule(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		module   string
		page     string
		wantName string
		wantRepo string
	}{
		{
			name:     "github",
			pkg:      "github.com/owner/tool",
			module:   "github.com/owner/tool",
			wantName: "tool",
			wantRepo: "https://github.com/owner/tool",
		},
		{
			name:     "major version",
			pkg:      "github.com/owner/tool/v2",
			module:   "github.com/owner/tool/v2",
			wantName: "tool",
			wantRepo: "https://github.com/owner/tool",
		},
		{
			name:     "not a major version",
			pkg:      "github.com/owner/tool/v1",
			module:   "github.com/owner/tool",
			wantName: "v1",
			wantRepo: "https://github.com/owner/tool",
		},
		{
			name:     "command in a subdirectory",
			pkg:      "github.com/owner/repo/cmd/tool",
			module:   "github.com/owner/repo",
			wantName: "tool",
			wantRepo: "https://github.com/owner/repo",
		},
		{
			name:     "module in a subdirectory",
			pkg:      "github.com/owner/repo/go/tool",
			module:   "github.com/owner/repo/go",
			wantName: "tool",
			wantRepo: "https://github.com/owner/repo",
		},
		{
			name:     "bitbucket",
			pkg:      "bitbucket.org/owner/tool/v3",
			module:   "bitbucket.org/owner/tool/v3",
			wantName: "tool",
			wantRepo: "https://bitbucket.org/owner/tool",
		},
		{
			name:     "vanity import path",
			pkg:      "example.com/tool",
			module:   "example.com/tool",
			page:     `<meta name="go-import" content="example.com/tool git https://github.com/owner/tool">`,
			wantName: "tool",
			wantRepo: "https://github.com/owner/tool",
		},
		{
			name:     "gitlab subgroup",
			pkg:      "gitlab.com/group/sub/tool/v2",
			module:   "gitlab.com/group/sub/tool/v2",
			page:     `<meta name="go-import" content="gitlab.com/group/sub/tool git https://gitlab.com/group/sub/tool.git">`,
			wantName: "tool",
			wantRepo: "https://gitlab.com/group/sub/tool",
		},
		{
			name:   "proxy entry and other prefixes",
			pkg:    "example.com/tool",
			module: "example.com/tool",
			page: `<meta name="go-import" content="example.com/tool mod https://proxy.example.com">
				<meta name="go-import" content="example.com/other git https://github.com/owner/other">
				<meta name="go-import" content="example.com/tool git https://github.com/owner/tool">`,
			wantName: "tool",
			wantRepo: "https://github.com/owner/tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func(rt http.RoundTripper) { client.Transport = rt }(client.Transport)
			client.Transport = page(tt.page)

			build := &debug.BuildInfo{Path: tt.pkg, Main: debug.Module{Path: tt.module, Version: "v1.2.3"}}
			r, err := resolve(t.Context(), Config{URL: "a"}, build)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if r.Name != tt.wantName || r.Repo != tt.wantRepo {
				t.Errorf("name, repo = %q, %q, want %q, %q", r.Name, r.Repo, tt.wantName, tt.wantRepo)
			}
		})
	}
}

type page string

func (p page) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(p)))}, nil
}
