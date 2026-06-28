package store

import (
	"context"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store/sqlc"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func toDocument(r sqlc.Document) domain.Document {
	return domain.Document{
		ID:        r.ID,
		UserID:    r.UserID,
		Title:     r.Title,
		Mode:      r.Mode,
		Status:    r.Status,
		PageCount: int(r.PageCount),
		CreatedAt: r.CreatedAt.Time,
	}
}

func toPage(r sqlc.Page) domain.Page {
	return domain.Page{
		ID:         r.ID,
		DocumentID: r.DocumentID,
		PageNumber: int(r.PageNumber),
		ImagePath:  r.ImagePath,
		ThumbPath:  r.ThumbPath,
		Width:      int(r.Width),
		Height:     int(r.Height),
		Status:     r.Status,
	}
}

func toOCRResult(r sqlc.OcrResult) domain.OCRResult {
	return domain.OCRResult{
		ID:        r.ID,
		PageID:    r.PageID,
		Model:     r.Model,
		Text:      r.Text,
		Status:    r.Status,
		CreatedAt: r.CreatedAt.Time,
	}
}

func toTemplate(r sqlc.Template) domain.Template {
	return domain.Template{
		ID:           r.ID,
		OwnerUserID:  r.OwnerUserID,
		Name:         r.Name,
		DocTypeHint:  r.DocTypeHint,
		Scope:        r.Scope,
		Prompt:       r.Prompt,
		OutputFormat: r.OutputFormat,
		JSONSchema:   r.JsonSchema,
		IsBuiltin:    r.IsBuiltin,
	}
}

func toOutput(r sqlc.Output) domain.Output {
	return domain.Output{
		ID:         r.ID,
		UserID:     r.UserID,
		DocumentID: r.DocumentID,
		TemplateID: r.TemplateID,
		Content:    r.Content,
		FilePath:   r.FilePath,
		Model:      r.Model,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt.Time,
	}
}

func (s *Store) CreateDocument(ctx context.Context, p domain.CreateDocumentParams) (domain.Document, error) {
	r, err := s.q.CreateDocument(ctx, sqlc.CreateDocumentParams{
		ID: p.ID, UserID: p.UserID, Title: p.Title, Mode: p.Mode,
	})
	if err != nil {
		return domain.Document{}, mapErr(err)
	}
	return toDocument(r), nil
}

func (s *Store) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	r, err := s.q.GetDocument(ctx, id)
	if err != nil {
		return domain.Document{}, mapErr(err)
	}
	return toDocument(r), nil
}

func (s *Store) ListDocumentsByUser(ctx context.Context, userID string) ([]domain.Document, error) {
	rows, err := s.q.ListDocumentsByUser(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Document, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDocument(r))
	}
	return out, nil
}

func (s *Store) SetDocumentStatus(ctx context.Context, id, status string) error {
	return mapErr(s.q.SetDocumentStatus(ctx, sqlc.SetDocumentStatusParams{ID: id, Status: status}))
}

func (s *Store) IncrementDocumentPageCount(ctx context.Context, id string) (int, error) {
	n, err := s.q.IncrementDocumentPageCount(ctx, id)
	if err != nil {
		return 0, mapErr(err)
	}
	return int(n), nil
}

func (s *Store) UpdateDocumentTitle(ctx context.Context, id, title string) (domain.Document, error) {
	r, err := s.q.UpdateDocumentTitle(ctx, sqlc.UpdateDocumentTitleParams{ID: id, Title: title})
	if err != nil {
		return domain.Document{}, mapErr(err)
	}
	return toDocument(r), nil
}

func (s *Store) SoftDeleteDocument(ctx context.Context, id string) error {
	return mapErr(s.q.SoftDeleteDocument(ctx, id))
}

func (s *Store) CreatePage(ctx context.Context, p domain.CreatePageParams) (domain.Page, error) {
	r, err := s.q.CreatePage(ctx, sqlc.CreatePageParams{
		ID: p.ID, DocumentID: p.DocumentID, PageNumber: int32(p.PageNumber),
		ImagePath: p.ImagePath, ThumbPath: p.ThumbPath,
		Width: int32(p.Width), Height: int32(p.Height), Status: p.Status,
	})
	if err != nil {
		return domain.Page{}, mapErr(err)
	}
	return toPage(r), nil
}

func (s *Store) GetPageByNumber(ctx context.Context, documentID string, pageNumber int) (domain.Page, error) {
	r, err := s.q.GetPageByNumber(ctx, sqlc.GetPageByNumberParams{
		DocumentID: documentID, PageNumber: int32(pageNumber),
	})
	if err != nil {
		return domain.Page{}, mapErr(err)
	}
	return toPage(r), nil
}

func (s *Store) ListPagesByDocument(ctx context.Context, documentID string) ([]domain.Page, error) {
	rows, err := s.q.ListPagesByDocument(ctx, documentID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Page, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPage(r))
	}
	return out, nil
}

func (s *Store) SetPageStatus(ctx context.Context, id, status string) error {
	return mapErr(s.q.SetPageStatus(ctx, sqlc.SetPageStatusParams{ID: id, Status: status}))
}

func (s *Store) ClearPageImage(ctx context.Context, id string) error {
	return mapErr(s.q.ClearPageImage(ctx, id))
}

func (s *Store) CreateOCRResult(ctx context.Context, p domain.CreateOCRResultParams) (domain.OCRResult, error) {
	r, err := s.q.CreateOCRResult(ctx, sqlc.CreateOCRResultParams{
		ID: p.ID, PageID: p.PageID, Model: p.Model, Text: p.Text, Status: p.Status,
	})
	if err != nil {
		return domain.OCRResult{}, mapErr(err)
	}
	return toOCRResult(r), nil
}

func (s *Store) GetLatestOCRResult(ctx context.Context, pageID string) (domain.OCRResult, error) {
	r, err := s.q.GetLatestOCRResult(ctx, pageID)
	if err != nil {
		return domain.OCRResult{}, mapErr(err)
	}
	return toOCRResult(r), nil
}

func (s *Store) ListTemplates(ctx context.Context) ([]domain.Template, error) {
	rows, err := s.q.ListTemplates(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTemplate(r))
	}
	return out, nil
}

func (s *Store) GetTemplate(ctx context.Context, id string) (domain.Template, error) {
	r, err := s.q.GetTemplate(ctx, id)
	if err != nil {
		return domain.Template{}, mapErr(err)
	}
	return toTemplate(r), nil
}

func (s *Store) CreateOutput(ctx context.Context, p domain.CreateOutputParams) (domain.Output, error) {
	r, err := s.q.CreateOutput(ctx, sqlc.CreateOutputParams{
		ID: p.ID, UserID: p.UserID, DocumentID: p.DocumentID, TemplateID: p.TemplateID,
		Content: p.Content, FilePath: p.FilePath, Model: p.Model, Status: p.Status,
	})
	if err != nil {
		return domain.Output{}, mapErr(err)
	}
	return toOutput(r), nil
}

func (s *Store) GetOutput(ctx context.Context, id string) (domain.Output, error) {
	r, err := s.q.GetOutput(ctx, id)
	if err != nil {
		return domain.Output{}, mapErr(err)
	}
	return toOutput(r), nil
}

func (s *Store) ListOutputsByDocument(ctx context.Context, documentID string) ([]domain.Output, error) {
	rows, err := s.q.ListOutputsByDocument(ctx, documentID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Output, 0, len(rows))
	for _, r := range rows {
		out = append(out, toOutput(r))
	}
	return out, nil
}
