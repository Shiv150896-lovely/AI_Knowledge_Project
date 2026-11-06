package models

import "time"

type Organization struct {
	ID, Name, Email string
	CreatedAt       time.Time
}
type Document struct {
	ID, Filename, Status string
	SizeBytes            int64
	PageCount            int
	CreatedAt            time.Time
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
	QuestionType string   `json:"question_type"`
	Category     string   `json:"category,omitempty"`
	Eligible     *bool    `json:"eligible,omitempty"`
	Score        float64  `json:"score,omitempty"`
	MissingFacts []string `json:"missing_facts,omitempty"`
}
