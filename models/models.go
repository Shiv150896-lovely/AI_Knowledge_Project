package models

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
type Document struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Status    string    `json:"status"`
	SizeBytes int64     `json:"size_bytes"`
	PageCount int       `json:"page_count"`
	CreatedAt time.Time `json:"created_at"`
}
type Reference struct {
	Document string `json:"document"`
	Page     int    `json:"page"`
}
type Chunk struct {
	DocumentID, Filename, Content string
	Page                          int
}
type RegisterRequest struct {
	OrganizationName string `json:"organization_name" binding:"required"`
	Email            string `json:"email" binding:"required,email"`
	Password         string `json:"password" binding:"required,min=8"`
}
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}
type AskRequest struct {
	Question string `json:"question" binding:"required"`
}
type Decision struct {
	QuestionType  string         `json:"question_type"`
	Category      string         `json:"category,omitempty"`
	RequiredFacts []string       `json:"required_facts,omitempty"`
	ProvidedFacts map[string]any `json:"provided_facts,omitempty"`
	Eligible      *bool          `json:"eligible,omitempty"`
	Score         float64        `json:"score,omitempty"`
	Reasons       []string       `json:"reasons,omitempty"`
	MissingFacts  []string       `json:"missing_facts,omitempty"`
}
