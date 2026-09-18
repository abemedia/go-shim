package shim

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// A Format is a release asset's archive or compression format.
type Format uint8

// Supported formats.
const (
	FormatAuto Format = iota // inferred from the asset's file extension
	FormatTarGz
	FormatTarBz2
	FormatTar
	FormatZip
	FormatGz
	FormatBz2
	FormatBinary // an uncompressed executable
)

func extract(name string, format Format, src io.Reader, paths []string, dst *os.File) error {
	if format == FormatAuto {
		ext := path.Ext(name)
		tarred := strings.HasSuffix(strings.TrimSuffix(name, ext), ".tar")

		switch {
		case ext == "", ext == ".exe":
			format = FormatBinary
		case ext == ".tar":
			format = FormatTar
		case ext == ".zip":
			format = FormatZip
		case ext == ".tgz", ext == ".gz" && tarred:
			format = FormatTarGz
		case ext == ".tbz", ext == ".tbz2", ext == ".bz2" && tarred:
			format = FormatTarBz2
		case ext == ".gz":
			format = FormatGz
		case ext == ".bz2":
			format = FormatBz2
		default:
			return errors.New("unsupported format")
		}
	}

	switch format {
	case FormatGz, FormatTarGz:
		r, err := gzip.NewReader(src)
		if err != nil {
			return err
		}
		src = r
	case FormatBz2, FormatTarBz2:
		src = bzip2.NewReader(src)
	}

	switch format {
	case FormatBinary, FormatGz, FormatBz2:
		_, err := io.Copy(dst, src)
		return err
	case FormatTar, FormatTarGz, FormatTarBz2:
		return untar(src, paths, dst)
	case FormatZip:
		return unzip(src, paths, dst)
	}
	return errors.New("invalid Config.Format")
}

func untar(src io.Reader, paths []string, dst *os.File) error {
	tr := tar.NewReader(src)
	best := len(paths)

	for {
		head, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		at := slices.Index(paths, path.Clean(strings.ReplaceAll(head.Name, `\`, "/")))
		regular := head.Typeflag == tar.TypeReg || head.Typeflag == tar.TypeGNUSparse
		if at < 0 || at >= best || !regular {
			continue
		}
		if _, err := dst.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := dst.Truncate(0); err != nil {
			return err
		}
		if _, err := io.Copy(dst, tr); err != nil {
			return err
		}
		best = at
	}

	// Drain src so the decompressor verifies its checksum.
	if _, err := io.Copy(io.Discard, src); err != nil {
		return err
	}
	if best == len(paths) {
		return errors.New("binary not found")
	}
	return nil
}

func unzip(src io.Reader, paths []string, dst *os.File) error {
	spool, err := os.CreateTemp(filepath.Dir(dst.Name()), ".shim-*")
	if err != nil {
		return err
	}
	defer os.Remove(spool.Name())
	defer spool.Close()

	size, err := io.Copy(spool, src)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(spool, size)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return err
	}

	files := make(map[string]*zip.File, len(zr.File))
	for _, file := range zr.File {
		if file.Mode().IsRegular() {
			files[path.Clean(strings.ReplaceAll(file.Name, `\`, "/"))] = file
		}
	}
	for _, path := range paths {
		file, ok := files[path]
		if !ok {
			continue
		}
		entry, err := file.Open()
		if err != nil {
			return err
		}
		defer entry.Close()
		_, err = io.Copy(dst, entry)
		return err
	}
	return errors.New("binary not found")
}
