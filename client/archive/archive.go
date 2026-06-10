package archive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Pack zips src (file or directory) into w.
// The zip preserves the top-level name so Unpack can restore it by
// extracting into filepath.Dir(src).
func Pack(src string, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return packFile(zw, src, info.Name())
	}

	// Use the parent as the base so the zip entry is "DirName/..."
	base := filepath.Dir(src)
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if fi.IsDir() {
			if rel != "." {
				_, err = zw.Create(rel + "/")
			}
			return err
		}
		return packFile(zw, path, rel)
	})
}

func packFile(zw *zip.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// Unpack extracts a zip archive (r of given size bytes) into dest.
// To restore a path that was packed from src, pass filepath.Dir(src) as dest.
func Unpack(r io.ReaderAt, size int64, dest string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range zr.File {
		target := filepath.Join(dest, filepath.FromSlash(f.Name))
		// Zip-slip guard
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
