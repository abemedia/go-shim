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

// A Format is how a release asset is packed.
type Format uint8

// The ways a release asset can be packed.
const (
	FormatAuto Format = iota // decided by the asset's name
	FormatTarGz
	FormatTarBz2
	FormatTar
	FormatZip
	FormatGz
	FormatBz2
	FormatBinary // the asset is the binary itself
)

//nolint:cyclop
func extract(name string, format Format, src io.Reader, paths []string, dst *os.File) error {
	if format == FormatAuto {
		// path.Ext of "x.tar.gz" is ".gz".
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
	case FormatBinary:
		_, err := io.Copy(dst, src)
		return err
	case FormatGz:
		r, err := gzip.NewReader(src)
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, r)
		return err
	case FormatBz2:
		_, err := io.Copy(dst, bzip2.NewReader(src))
		return err
	case FormatTar:
		return untar(src, paths, dst)
	case FormatTarGz:
		r, err := gzip.NewReader(src)
		if err != nil {
			return err
		}
		return untar(r, paths, dst)
	case FormatTarBz2:
		return untar(bzip2.NewReader(src), paths, dst)
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
		at := slices.Index(paths, head.Name)
		if at < 0 || at >= best {
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

	// The tar reader stops at the end-of-archive marker, but the compression checksum is only
	// verified at the end of the stream.
	if _, err := io.Copy(io.Discard, src); err != nil {
		return err
	}
	if best == len(paths) {
		return errors.New("binary not found")
	}
	return nil
}

func unzip(src io.Reader, paths []string, dst *os.File) error {
	// A zip's index is at its end, so it cannot be read from a stream.
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
	if err != nil {
		return err
	}

	files := make(map[string]*zip.File, len(zr.File))
	for _, file := range zr.File {
		files[file.Name] = file
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
