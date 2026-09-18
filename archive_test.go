package shim

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestExtract(t *testing.T) {
	const binary = "\x7fELF not really, but distinctive enough"

	raw := []byte(binary)
	tarGz := archive(t, FormatTarGz, fstest.MapFS{"tool": {Data: raw}})
	tarBz2 := fixture(t, "tool.tar.bz2")

	tests := []struct {
		asset    string
		override Format
		packed   []byte
		want     string
		wantErr  string
	}{
		{asset: "tool-1.2.3-x86_64-apple-darwin.tar.gz", packed: tarGz, want: binary},
		{asset: "tool.tgz", packed: tarGz, want: binary},
		{asset: "tool.tar", packed: archive(t, FormatTar, fstest.MapFS{"tool": {Data: raw}}), want: binary},
		{asset: "tool-x86_64-pc-windows-msvc.zip", packed: archive(t, FormatZip, fstest.MapFS{"tool": {Data: raw}}), want: binary},
		{asset: "tool.gz", packed: archive(t, FormatGz, fstest.MapFS{"tool": {Data: raw}}), want: binary},
		{asset: "tool", packed: raw, want: binary},
		{asset: "tool-linux-amd64", packed: raw, want: binary},
		{asset: "tool.exe", packed: raw, want: binary},
		// The standard library has no bzip2 writer, so these are committed fixtures.
		{asset: "tool-1.2.3.tar.bz2", packed: tarBz2, want: binary},
		{asset: "tool.tbz2", packed: tarBz2, want: binary},
		{asset: "tool.tbz", packed: tarBz2, want: binary},
		{asset: "tool.bz2", packed: fixture(t, "tool.bz2"), want: binary},

		{asset: "tool-1.2.3.tar.xz", override: FormatTarGz, packed: tarGz, want: binary},
		{asset: "tool-1.2.3.AppImage", override: FormatBinary, packed: raw, want: binary},
		{asset: "tool-linux.tar.gz", override: FormatBinary, packed: raw, want: binary},

		{asset: "nested.tar.gz", packed: archive(t, FormatTarGz, fstest.MapFS{"tool-1.2.3/tool": {Data: raw}}), want: binary},
		{asset: "nested.zip", packed: archive(t, FormatZip, fstest.MapFS{"tool-1.2.3/tool": {Data: raw}}), want: binary},
		{
			asset: "precedence.tar",
			// AddFS walks lexically, so the last-choice entry is written first.
			packed: archive(t, FormatTar, fstest.MapFS{
				"tool":            {Data: []byte("root")},
				"tool-1.2.3/tool": {Data: []byte("preferred")},
			}),
			want: "preferred",
		},

		{asset: "missing.tar.gz", packed: archive(t, FormatTarGz, fstest.MapFS{"other": {}}), wantErr: "binary not found"},
		{asset: "missing.zip", packed: archive(t, FormatZip, fstest.MapFS{"other": {}}), wantErr: "binary not found"},

		{asset: "tool-1.2.3-linux-amd64", wantErr: "unsupported format"},
		{asset: "tool-v1.0", wantErr: "unsupported format"},
		{asset: "tool.xz", wantErr: "unsupported format"},
		{asset: "tool.tar.xz", wantErr: "unsupported format"},
		{asset: "tool.zst", wantErr: "unsupported format"},
		{asset: "tool.7z", wantErr: "unsupported format"},
		{asset: "tool_1.2.3_amd64.deb", wantErr: "unsupported format"},
		{asset: "tool.AppImage", wantErr: "unsupported format"},
		{asset: "tool.tar.gz", override: Format(99), wantErr: "invalid Config.Format"},
	}

	for _, tt := range tests {
		t.Run(tt.asset, func(t *testing.T) {
			got, err := unpack(t, tt.asset, tt.override, tt.packed, "tool-1.2.3/tool", "tool")
			switch {
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("unpack error = %v, want %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unpack: %v", err)
			case string(got) != tt.want:
				t.Errorf("unpack returned %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractSkipsSymlinks(t *testing.T) {
	const binary = "\x7fELF not really, but distinctive enough"

	var tarred bytes.Buffer
	tw := tar.NewWriter(&tarred)
	if err := tw.WriteHeader(&tar.Header{Name: "tool-1.2.3/tool", Typeflag: tar.TypeSymlink, Linkname: "../tool"}); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "tool", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(binary))}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(tw, binary); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	var zipped bytes.Buffer
	zw := zip.NewWriter(&zipped)
	link := &zip.FileHeader{Name: "tool-1.2.3/tool"}
	link.SetMode(fs.ModeSymlink | 0o777)
	w, err := zw.CreateHeader(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "../tool"); err != nil {
		t.Fatal(err)
	}
	if w, err = zw.Create("tool"); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, binary); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	for asset, packed := range map[string][]byte{"tool.tar": tarred.Bytes(), "tool.zip": zipped.Bytes()} {
		t.Run(asset, func(t *testing.T) {
			got, err := unpack(t, asset, FormatAuto, packed, "tool-1.2.3/tool", "tool")
			if err != nil {
				t.Fatalf("unpack: %v", err)
			}
			if string(got) != binary {
				t.Errorf("unpack returned %q, want %q", got, binary)
			}
		})
	}
}

func TestExtractRejectsCorruption(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		asset  []byte
	}{
		{"tar.gz", FormatTarGz, archive(t, FormatTarGz, fstest.MapFS{"tool": {Data: bytes.Repeat([]byte("payload"), 4096)}})},
		{"tar.bz2", FormatTarBz2, fixture(t, "tool.tar.bz2")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, at := range []int{len(tt.asset) / 8, len(tt.asset) / 2, len(tt.asset) - 5} {
				corrupt := bytes.Clone(tt.asset)
				corrupt[at] ^= 0x01
				if _, err := unpack(t, "asset", tt.format, corrupt, "tool"); err == nil {
					t.Errorf("unpack accepted a bit flip at byte %d of %d", at, len(tt.asset))
				}
			}
		})
	}
}

func archive(t *testing.T, format Format, files fstest.MapFS) []byte {
	t.Helper()
	var buf bytes.Buffer

	switch format {
	case FormatZip:
		zw := zip.NewWriter(&buf)
		if err := zw.AddFS(files); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
	case FormatTar, FormatTarGz:
		tw := tar.NewWriter(&buf)
		if err := tw.AddFS(files); err != nil {
			t.Fatal(err)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
	case FormatGz:
		for _, f := range files {
			buf.Write(f.Data)
		}
	default:
		t.Fatalf("archive: cannot build %d", format)
	}

	if format != FormatTarGz && format != FormatGz {
		return buf.Bytes()
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return gz.Bytes()
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func unpack(t *testing.T, name string, override Format, asset []byte, paths ...string) ([]byte, error) {
	t.Helper()
	dst, err := os.CreateTemp(t.TempDir(), "binary-*")
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	if err := extract(name, override, bytes.NewReader(asset), paths, dst); err != nil {
		return nil, err
	}
	return os.ReadFile(dst.Name())
}
