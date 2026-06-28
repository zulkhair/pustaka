package domain

import "context"

type Mailer interface {
	SendVerificationCode(ctx context.Context, toEmail, code string) error
}

type BlobStore interface {
	Put(userID, docID string, page int, data []byte) (path string, err error)
	Get(path string) ([]byte, error)
	Delete(path string) error
	Thumbnail(userID, docID string, page int, data []byte) (path string, err error)
}

type AIClient interface {
	Transcribe(ctx context.Context, imageBytes []byte) (markdown string, err error)
	Transform(ctx context.Context, ocrText string, tmpl Template) (output string, err error)
}

type Store interface {
	ExecTx(ctx context.Context, fn func(Store) error) error

	CreateUser(ctx context.Context, p CreateUserParams) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	SetUserEmailVerified(ctx context.Context, id string) error

	CreateEmailVerification(ctx context.Context, p CreateEmailVerificationParams) (EmailVerification, error)
	GetActiveEmailVerification(ctx context.Context, userID string) (EmailVerification, error)
	IncrementVerificationAttempts(ctx context.Context, id string) (int, error)
	ConsumeEmailVerification(ctx context.Context, id string) error
	DeleteEmailVerificationsByUser(ctx context.Context, userID string) error

	CreateSession(ctx context.Context, p CreateSessionParams) (Session, error)
	GetSessionByTokenHash(ctx context.Context, hash string) (Session, error)
	RevokeSession(ctx context.Context, id string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error

	CreateDocument(ctx context.Context, p CreateDocumentParams) (Document, error)
	GetDocument(ctx context.Context, id string) (Document, error)
	ListDocumentsByUser(ctx context.Context, userID string) ([]Document, error)
	SetDocumentStatus(ctx context.Context, id, status string) error
	IncrementDocumentPageCount(ctx context.Context, id string) (int, error)
	UpdateDocumentTitle(ctx context.Context, id, title string) (Document, error)
	SetDocumentThumbPage(ctx context.Context, id string, page int) (Document, error)
	SoftDeleteDocument(ctx context.Context, id string) error

	CreatePage(ctx context.Context, p CreatePageParams) (Page, error)
	GetPageByNumber(ctx context.Context, documentID string, pageNumber int) (Page, error)
	ListPagesByDocument(ctx context.Context, documentID string) ([]Page, error)
	SetPageStatus(ctx context.Context, id, status string) error
	ClearPageImage(ctx context.Context, id string) error

	CreateOCRResult(ctx context.Context, p CreateOCRResultParams) (OCRResult, error)
	GetLatestOCRResult(ctx context.Context, pageID string) (OCRResult, error)

	ListTemplates(ctx context.Context) ([]Template, error)
	GetTemplate(ctx context.Context, id string) (Template, error)

	CreateOutput(ctx context.Context, p CreateOutputParams) (Output, error)
	GetOutput(ctx context.Context, id string) (Output, error)
	ListOutputsByDocument(ctx context.Context, documentID string) ([]Output, error)

	// Document sharing (Plan 3).
	CreateShare(ctx context.Context, p CreateShareParams) (DocumentShare, error)
	ListSharesForDocument(ctx context.Context, documentID string) ([]DocumentShare, error)
	GetShare(ctx context.Context, documentID, userID string) (DocumentShare, error)
	DeleteShare(ctx context.Context, documentID, userID string) error
	ListDocumentsSharedWith(ctx context.Context, userID string) ([]Document, error)
}
