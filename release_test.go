package shim

import (
	"path/filepath"
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
		want   []string
	}{
		{
			name: "one candidate per target name",
			config: Config{
				URL:     "{{.Name}}-{{.Version}}-{{.Target}}.tar.gz",
				Targets: []Target{{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Names: []string{"musl", "gnu"}}},
			},
			build: &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want: []string{
				"https://github.com/owner/tool/releases/download/v1.2.3/tool-1.2.3-musl.tar.gz",
				"https://github.com/owner/tool/releases/download/v1.2.3/tool-1.2.3-gnu.tar.gz",
			},
		},
		{
			name:   "a whole address is left alone",
			config: Config{URL: "{{.Repo}}/uploads/{{.Tag}}/{{.Name}}_{{.GOOS}}_{{.GOARCH}}"},
			build:  &debug.BuildInfo{Main: debug.Module{Path: "example.com/owner/tool", Version: "v1"}},
			want:   []string{"https://example.com/owner/tool/uploads/v1/tool_" + runtime.GOOS + "_" + runtime.GOARCH},
		},
		{
			name:   "Config.Version wins over the stamped version",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz", Version: "v9"},
			build:  &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://github.com/owner/tool/releases/download/v9/tool-9.tar.gz"},
		},
		{
			name:   "github",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://github.com/owner/tool/releases/download/v1.2.3/tool-1.2.3.tar.gz"},
		},
		{
			name:   "codeberg",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Main: debug.Module{Path: "codeberg.org/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://codeberg.org/owner/tool/releases/download/v1.2.3/tool-1.2.3.tar.gz"},
		},
		{
			name:   "gitlab",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Main: debug.Module{Path: "gitlab.com/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://gitlab.com/owner/tool/-/releases/v1.2.3/downloads/tool-1.2.3.tar.gz"},
		},
		{
			name:   "bitbucket",
			config: Config{URL: "{{.Name}}-{{.Version}}.tar.gz"},
			build:  &debug.BuildInfo{Main: debug.Module{Path: "bitbucket.org/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://bitbucket.org/owner/tool/downloads/tool-1.2.3.tar.gz"},
		},
		{
			name: "build settings",
			config: Config{
				URL: "https://example.com/{{.GO386}}|{{.GOAMD64}}|{{.GOARM}}|{{.GOARM64}}|" +
					"{{.GOMIPS}}|{{.GOMIPS64}}|{{.GOPPC64}}|{{.GORISCV64}}",
			},
			build: &debug.BuildInfo{
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
			build:  &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			want:   []string{"https://example.com/x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := resolve(tt.config, tt.build)
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
			build := &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}}
			r, err := resolve(tt.config, build)
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
		wantErr string
	}{
		{
			name:    "missing url",
			config:  Config{},
			build:   &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "Config.URL",
		},
		{
			name:    "unparsable url",
			config:  Config{URL: "{{"},
			build:   &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "Config.URL",
		},
		{
			name:    "unknown field",
			config:  Config{URL: "{{.Nope}}"},
			build:   &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "Config.URL",
		},
		{
			name:    "unknown platform",
			config:  Config{URL: "a", Targets: []Target{{GOOS: "plan9", GOARCH: "386", Names: []string{"x"}}}},
			build:   &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}},
			wantErr: "no v1.2.3 release for " + runtime.GOOS,
		},
		{
			name:    "built from source",
			config:  Config{URL: "a"},
			build:   &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "(devel)"}},
			wantErr: "built from source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolve(tt.config, tt.build)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("resolve error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestResolveCachePath(t *testing.T) {
	build := &debug.BuildInfo{Main: debug.Module{Path: "github.com/owner/tool", Version: "v1.2.3"}}
	r, err := resolve(Config{URL: "{{.Name}}{{.Ext}}"}, build)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	bin := "tool"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if want := filepath.Join("go-shim", "github.com", "owner", "tool@v1.2.3", bin); !strings.HasSuffix(r.bin, want) {
		t.Errorf("cached at %q, want a path ending in %q", r.bin, want)
	}
}

func TestReleasePage(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"github.com", "{{.Repo}}/releases/download/{{.Tag}}/"},
		{"codeberg.org", "{{.Repo}}/releases/download/{{.Tag}}/"},
		{"gitea.example.com", "{{.Repo}}/releases/download/{{.Tag}}/"},
		{"gitlab.com", "{{.Repo}}/-/releases/{{.Tag}}/downloads/"},
		{"bitbucket.org", "{{.Repo}}/downloads/"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := releasePage(tt.host); got != tt.want {
				t.Errorf("releasePage(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestTrimMajor(t *testing.T) {
	tests := []struct {
		module string
		want   string
	}{
		{"github.com/owner/tool", "github.com/owner/tool"},
		{"github.com/owner/tool/v2", "github.com/owner/tool"},
		{"github.com/owner/tool/v10", "github.com/owner/tool"},
		{"github.com/owner/tool/vi", "github.com/owner/tool/vi"},
		{"github.com/owner/v2", "github.com/owner"},
		{"example.com/tool/verbose", "example.com/tool/verbose"},
		{"v2", "v2"},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			if got := trimMajor(tt.module); got != tt.want {
				t.Errorf("trimMajor(%q) = %q, want %q", tt.module, got, tt.want)
			}
		})
	}
}
