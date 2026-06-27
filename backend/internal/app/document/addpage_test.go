package document_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/blob"
	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

type fakeRunner struct {
	text  string
	calls int
}

func (f *fakeRunner) Run(_ context.Context, page domain.Page, _ []byte) (domain.OCRResult, error) {
	f.calls++
	return domain.OCRResult{PageID: page.ID, Model: "fake", Text: f.text, Status: domain.StatusDone}, nil
}

func TestAddPagePhotoMode(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	bs := blob.NewMemory()
	svc := document.New(st, bs)
	ctx := context.Background()
	uid := newUser(t, st)

	doc, err := svc.Create(ctx, uid, "Photos", domain.ModePhoto)
	require.NoError(t, err)

	runner := &fakeRunner{text: "# page one"}
	res, err := svc.AddPage(ctx, uid, doc.ID, []byte("img-bytes"), runner)
	require.NoError(t, err)
	require.Equal(t, 1, res.Page.PageNumber)
	require.NotNil(t, res.Page.ImagePath)
	require.NotNil(t, res.Page.ThumbPath)
	require.Equal(t, "# page one", res.OCRText)
	require.Equal(t, 1, runner.calls)

	// image is retrievable from blob in photo mode
	got, err := bs.Get(*res.Page.ImagePath)
	require.NoError(t, err)
	require.Equal(t, []byte("img-bytes"), got)

	// page_count incremented
	d, err := st.GetDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Equal(t, 1, d.PageCount)
}

func TestAddPageTextModeDiscardsImage(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	bs := blob.NewMemory()
	svc := document.New(st, bs)
	ctx := context.Background()
	uid := newUser(t, st)

	doc, err := svc.Create(ctx, uid, "Text", domain.ModeText)
	require.NoError(t, err)

	res, err := svc.AddPage(ctx, uid, doc.ID, []byte("img"), &fakeRunner{text: "transcribed"})
	require.NoError(t, err)
	require.Nil(t, res.Page.ImagePath, "text mode must not persist the image")
	require.Equal(t, "transcribed", res.OCRText)
}

func TestAddPageOwnerScoped(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := document.New(st, blob.NewMemory())
	ctx := context.Background()
	owner := newUser(t, st)
	other := newUser(t, st)
	doc, err := svc.Create(ctx, owner, "D", domain.ModePhoto)
	require.NoError(t, err)

	_, err = svc.AddPage(ctx, other, doc.ID, []byte("img"), &fakeRunner{text: "x"})
	require.ErrorIs(t, err, domain.ErrNotFound)
}
