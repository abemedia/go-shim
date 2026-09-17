package shim

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownload(t *testing.T) {
	tests := []struct {
		name     string
		serve    map[string]int // path to status; anything else is a 404
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
			name:     "forbidden is not absent",
			serve:    map[string]int{"/denied": http.StatusForbidden},
			paths:    []string{"/denied"},
			wantReqs: 1,
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

			client.Transport = srv.Client().Transport
			defer func() { client.Transport = nil }()

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

	client.Transport = redirect.Client().Transport
	defer func() { client.Transport = nil }()

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
