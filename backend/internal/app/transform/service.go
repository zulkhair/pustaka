package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

const transformModel = "qwen2.5:14b-instruct"

type Service struct {
	store domain.Store
	ai    domain.AIClient
	authz document.Authorizer
}

func New(store domain.Store, ai domain.AIClient, authz document.Authorizer) *Service {
	return &Service{store: store, ai: ai, authz: authz}
}

type pageEntry struct {
	PageNumber int             `json:"page_number"`
	Result     json.RawMessage `json:"result"`
}

func (s *Service) Run(ctx context.Context, userID, docID, templateID string) (domain.Output, error) {
	if _, err := s.authz.AuthorizeDoc(ctx, userID, docID, document.PermWrite); err != nil {
		return domain.Output{}, err
	}

	tmpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil {
		return domain.Output{}, err
	}

	pages, err := s.store.ListPagesByDocument(ctx, docID)
	if err != nil {
		return domain.Output{}, err
	}

	// Collect each page's latest OCR text.
	type pageOCR struct {
		number int
		text   string
	}
	var ocrs []pageOCR
	for _, p := range pages {
		res, err := s.store.GetLatestOCRResult(ctx, p.ID)
		if err != nil {
			continue // page not yet OCR'd; skip
		}
		if res.Status == domain.StatusDone && strings.TrimSpace(res.Text) != "" {
			ocrs = append(ocrs, pageOCR{number: p.PageNumber, text: res.Text})
		}
	}
	if len(ocrs) == 0 {
		return domain.Output{}, domain.ErrValidation
	}

	var content string
	switch tmpl.Scope {
	case domain.ScopePage:
		entries := make([]pageEntry, 0, len(ocrs))
		for _, o := range ocrs {
			res, err := s.ai.Transform(ctx, o.text, tmpl)
			if err != nil {
				return domain.Output{}, err
			}
			entries = append(entries, pageEntry{PageNumber: o.number, Result: rawOrString(res, tmpl.OutputFormat)})
		}
		buf, err := json.Marshal(entries)
		if err != nil {
			return domain.Output{}, err
		}
		content = string(buf)
	case domain.ScopeDocument:
		var sb strings.Builder
		for _, o := range ocrs {
			fmt.Fprintf(&sb, "\n\n--- PAGE %d ---\n%s", o.number, o.text)
		}
		res, err := s.ai.Transform(ctx, sb.String(), tmpl)
		if err != nil {
			return domain.Output{}, err
		}
		content = res
	default:
		return domain.Output{}, domain.ErrValidation
	}

	return s.store.CreateOutput(ctx, domain.CreateOutputParams{
		ID: uuid.NewString(), UserID: userID, DocumentID: docID, TemplateID: tmpl.ID,
		Content: content, Model: transformModel, Status: domain.StatusDone,
	})
}

// rawOrString wraps the per-page result as JSON: parseable JSON stays as-is,
// anything else becomes a JSON string.
func rawOrString(res, format string) json.RawMessage {
	if format == domain.FormatJSON && json.Valid([]byte(res)) {
		return json.RawMessage(res)
	}
	b, _ := json.Marshal(res)
	return json.RawMessage(b)
}

func (s *Service) GetOutput(ctx context.Context, userID, outputID string) (domain.Output, error) {
	out, err := s.store.GetOutput(ctx, outputID)
	if err != nil {
		return domain.Output{}, err
	}
	// Authorize against the output's parent document so a sharee may read it,
	// identically to pages/OCR/images. A non-owner with no share gets ErrNotFound.
	if _, err := s.authz.AuthorizeDoc(ctx, userID, out.DocumentID, document.PermRead); err != nil {
		return domain.Output{}, err
	}
	return out, nil
}
