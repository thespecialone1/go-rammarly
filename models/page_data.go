package models

// PageData holds data passed to the HTML template.
type PageData struct {
	CurrentPage   string
	OriginalText  string
	CorrectedText string
	ImageAnalysis string
	User          *User
}

// User represents a logged-in user.
type User struct {
	ID      int64
	Email   string
	Name    string
	Picture string
}
