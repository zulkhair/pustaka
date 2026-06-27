package domain

type Page struct {
	ID         string
	DocumentID string
	PageNumber int
	ImagePath  *string
	ThumbPath  *string
	Width      int
	Height     int
	Status     string
}

type CreatePageParams struct {
	ID         string
	DocumentID string
	PageNumber int
	ImagePath  *string
	ThumbPath  *string
	Width      int
	Height     int
	Status     string
}
