package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestNewPortsAndErrors(t *testing.T) {
	var _ domain.BlobStore = stubBlob{}
	var _ domain.AIClient = stubAI{}
	require.NotNil(t, domain.ErrUnsupportedFormat)
	require.NotNil(t, domain.ErrSchemaInvalid)
	require.NotEqual(t, domain.ErrUnsupportedFormat, domain.ErrSchemaInvalid)
	require.NotEqual(t, domain.ErrValidation, domain.ErrUnsupportedFormat)
}

type stubBlob struct{}

func (stubBlob) Put(string, string, int, []byte) (string, error)       { return "", nil }
func (stubBlob) Get(string) ([]byte, error)                            { return nil, nil }
func (stubBlob) Delete(string) error                                   { return nil }
func (stubBlob) Thumbnail(string, string, int, []byte) (string, error) { return "", nil }

type stubAI struct{}

func (stubAI) Transcribe(context.Context, []byte) (string, error)                  { return "", nil }
func (stubAI) Transform(context.Context, string, domain.Template) (string, error) { return "", nil }
