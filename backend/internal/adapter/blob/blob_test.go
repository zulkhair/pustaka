package blob_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/blob"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 100, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))
	return buf.Bytes()
}

func TestFSPutGetDelete(t *testing.T) {
	dir := t.TempDir()
	var fs domain.BlobStore = blob.New(dir)
	data := makeJPEG(t, 3000, 1500) // larger than the 2048 clamp

	path, err := fs.Put("user1", "doc1", 2, data)
	require.NoError(t, err)
	require.Equal(t, filepath.ToSlash(filepath.Join("user1", "doc1", "2.jpg")), path)
	require.FileExists(t, filepath.Join(dir, "user1", "doc1", "2.jpg"))

	got, err := fs.Get(path)
	require.NoError(t, err)
	require.NotEmpty(t, got)

	// re-decode to confirm it is a valid jpeg clamped to <= 2048 on the long edge
	img, _, err := image.Decode(bytes.NewReader(got))
	require.NoError(t, err)
	b := img.Bounds()
	require.LessOrEqual(t, b.Dx(), 2048)
	require.LessOrEqual(t, b.Dy(), 2048)

	require.NoError(t, fs.Delete(path))
	_, err = os.Stat(filepath.Join(dir, "user1", "doc1", "2.jpg"))
	require.True(t, os.IsNotExist(err))
}

func TestFSThumbnail(t *testing.T) {
	dir := t.TempDir()
	fs := blob.New(dir)
	data := makeJPEG(t, 2000, 1000)

	tpath, err := fs.Thumbnail("u", "d", 1, data)
	require.NoError(t, err)
	require.Equal(t, filepath.ToSlash(filepath.Join("u", "d", "1_thumb.jpg")), tpath)

	raw, err := fs.Get(tpath)
	require.NoError(t, err)
	img, _, err := image.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	require.LessOrEqual(t, img.Bounds().Dx(), 400)
	require.LessOrEqual(t, img.Bounds().Dy(), 400)
}

func TestMemoryBlob(t *testing.T) {
	var m domain.BlobStore = blob.NewMemory()
	p, err := m.Put("u", "d", 1, []byte("raw-bytes"))
	require.NoError(t, err)
	got, err := m.Get(p)
	require.NoError(t, err)
	require.Equal(t, []byte("raw-bytes"), got)

	tp, err := m.Thumbnail("u", "d", 1, []byte("thumb-bytes"))
	require.NoError(t, err)
	require.NotEqual(t, p, tp)

	require.NoError(t, m.Delete(p))
	_, err = m.Get(p)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
