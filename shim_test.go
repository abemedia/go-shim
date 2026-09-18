package shim

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	build := exec.CommandContext(t.Context(), "go", "build", "-o", dir, "./testdata/tool", "./testdata/shim")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	var ext string
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	asset, err := os.ReadFile(filepath.Join(dir, "tool"+ext))
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "shim"+ext)
	shim, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}

	var requests int
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Write(asset)
	}))
	defer srv.Close()

	ca := filepath.Join(dir, "ca.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(ca, cert, 0o600); err != nil {
		t.Fatal(err)
	}

	// Run twice, because on Windows the first replacement cannot remove the binary it moved aside.
	for i := range 2 {
		if err := os.WriteFile(exe, shim, 0o755); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command(exe, "--flag", "arg") //nolint:noctx
		cmd.Env = append(os.Environ(), "SHIM_TEST_URL="+srv.URL+"/tool", "SHIM_TEST_CA="+ca)
		out, err := cmd.CombinedOutput()

		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run %d: %v\n%s", i, err, out)
		}
		if exit.ExitCode() != 3 {
			t.Errorf("run %d: exit = %d, want 3\n%s", i, exit.ExitCode(), out)
		}
		if want := "go-shim-test-tool [--flag arg]"; !strings.Contains(string(out), want) {
			t.Errorf("run %d: output %q does not contain %q", i, out, want)
		}
		if replaced, err := os.ReadFile(exe); err != nil {
			t.Fatal(err)
		} else if !bytes.Equal(replaced, asset) {
			t.Errorf("run %d: the binary was not replaced with the release", i)
		}
	}

	if requests != 2 {
		t.Errorf("made %d requests, want 2", requests)
	}
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name     string
		serve    map[string]int
		paths    []string
		cancel   bool
		wantPath string
		wantReqs int
		wantErr  string
	}{
		{
			name:     "takes the first match",
			serve:    map[string]int{"/musl": http.StatusOK, "/gnu": http.StatusOK},
			paths:    []string{"/musl", "/gnu"},
			wantPath: "/musl",
			wantReqs: 1,
		},
		{
			name:     "falls back to a later candidate",
			serve:    map[string]int{"/gnu": http.StatusOK},
			paths:    []string{"/musl", "/gnu"},
			wantPath: "/gnu",
			wantReqs: 2,
		},
		{
			name:     "nothing published",
			paths:    []string{"/musl", "/gnu"},
			wantReqs: 2,
			wantErr:  "no v1 release for",
		},
		{
			name:     "forbidden falls back to a later candidate",
			serve:    map[string]int{"/musl": http.StatusForbidden, "/gnu": http.StatusOK},
			paths:    []string{"/musl", "/gnu"},
			wantPath: "/gnu",
			wantReqs: 2,
		},
		{
			name:     "forbidden is reported when nothing is published",
			serve:    map[string]int{"/musl": http.StatusForbidden},
			paths:    []string{"/musl", "/gnu"},
			wantReqs: 2,
			wantErr:  "403",
		},
		{
			name:    "cancelled context",
			serve:   map[string]int{"/musl": http.StatusOK},
			paths:   []string{"/musl"},
			cancel:  true,
			wantErr: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				status, ok := tt.serve[r.URL.Path]
				if !ok {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(status)
				w.Write([]byte("asset"))
			}))
			defer srv.Close()

			defer func(rt http.RoundTripper) { client.Transport = rt }(client.Transport)
			client.Transport = httpsOnly{srv.Client().Transport}

			ctx := context.Background()
			if tt.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			r := &release{Name: "tool", Tag: "v1"}
			for _, p := range tt.paths {
				r.assets = append(r.assets, asset{url: srv.URL + p})
			}

			got, body, err := download(ctx, r)
			if body != nil {
				defer body.Close()
			}

			switch {
			case tt.wantErr != "":
				if err == nil {
					t.Fatalf("download succeeded, want an error mentioning %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not mention %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("download: %v", err)
			default:
				if !strings.HasSuffix(got.url, tt.wantPath) {
					t.Errorf("downloaded %q, want %q", got.url, tt.wantPath)
				}
				if data, _ := io.ReadAll(body); string(data) != "asset" {
					t.Errorf("body = %q, want %q", data, "asset")
				}
			}
			if requests != tt.wantReqs {
				t.Errorf("made %d requests, want %d", requests, tt.wantReqs)
			}
		})
	}
}

func TestDownloadRejectsPlaintext(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("made a plaintext request for %s", r.URL.Path)
	}))
	defer plain.Close()

	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirect.Close()

	defer func(rt http.RoundTripper) { client.Transport = rt }(client.Transport)
	client.Transport = httpsOnly{redirect.Client().Transport}

	for _, tt := range []struct{ name, url string }{
		{"plain candidate", plain.URL + "/musl"},
		{"redirect to plain", redirect.URL + "/musl"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &release{Name: "tool", Tag: "v1", assets: []asset{{url: tt.url}}}
			_, _, err := download(context.Background(), r)
			if err == nil || !strings.Contains(err.Error(), "not https") {
				t.Errorf("download error = %v, want a not https error", err)
			}
		})
	}
}
