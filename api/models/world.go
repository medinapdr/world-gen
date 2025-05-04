package models

import "time"

type World struct {
	ID          int       `json:"id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Population  int       `json:"population"`
	Climate     string    `json:"climate"`
	Features    []string  `json:"features"`
	Theme       string    `json:"theme"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	Fauna       []string  `json:"fauna,omitempty"`
	Flora       []string  `json:"flora,omitempty"`
	Cultures    []string  `json:"cultures,omitempty"`
	Dangers     []string  `json:"dangers,omitempty"`
	Languages   []string  `json:"languages,omitempty"`
}

// PaginatedWorldsResponse represents a paginated list of worlds with metadata
type PaginatedWorldsResponse struct {
	Data   []World `json:"data"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}
