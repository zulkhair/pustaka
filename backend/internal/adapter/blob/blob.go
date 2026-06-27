package blob

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

const (
	maxEdge   = 2048
	thumbEdge = 400
	jpegQual  = 80
)

// FS is a filesystem-backed domain.BlobStore. Files live under baseDir; the
// paths returned/accepted are RELATIVE to baseDir (forward-slash separated).
type FS struct {
	baseDir string
}

func New(baseDir string) *FS { return &FS{baseDir: baseDir} }

var _ domain.BlobStore = (*FS)(nil)

func relPath(userID, docID string, page int, suffix string) string {
	return filepath.ToSlash(filepath.Join(userID, docID, fmt.Sprintf("%d%s.jpg", page, suffix)))
}

func (f *FS) write(rel string, data []byte) error {
	abs := filepath.Join(f.baseDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("blob: mkdir: %w", err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return fmt.Errorf("blob: write: %w", err)
	}
	return nil
}

// normalize decodes, clamps the longest edge to longest (no upscaling), and
// re-encodes as JPEG.
func normalize(data []byte, longest, quality int) ([]byte, error) {
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("blob: decode image: %w", err)
	}
	img = clamp(img, longest)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
		return nil, fmt.Errorf("blob: encode image: %w", err)
	}
	return buf.Bytes(), nil
}

func clamp(img image.Image, longest int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= longest && h <= longest {
		return img
	}
	if w >= h {
		return imaging.Resize(img, longest, 0, imaging.Lanczos)
	}
	return imaging.Resize(img, 0, longest, imaging.Lanczos)
}

func (f *FS) Put(userID, docID string, page int, data []byte) (string, error) {
	norm, err := normalize(data, maxEdge, jpegQual)
	if err != nil {
		return "", err
	}
	rel := relPath(userID, docID, page, "")
	if err := f.write(rel, norm); err != nil {
		return "", err
	}
	return rel, nil
}

func (f *FS) Thumbnail(userID, docID string, page int, data []byte) (string, error) {
	norm, err := normalize(data, thumbEdge, jpegQual)
	if err != nil {
		return "", err
	}
	rel := relPath(userID, docID, page, "_thumb")
	if err := f.write(rel, norm); err != nil {
		return "", err
	}
	return rel, nil
}

func (f *FS) Get(rel string) ([]byte, error) {
	abs := filepath.Join(f.baseDir, filepath.FromSlash(rel))
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("blob: read: %w", err)
	}
	return data, nil
}

func (f *FS) Delete(rel string) error {
	abs := filepath.Join(f.baseDir, filepath.FromSlash(rel))
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("blob: delete: %w", err)
	}
	return nil
}
