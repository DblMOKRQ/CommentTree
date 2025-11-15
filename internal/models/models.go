package models

import "time"

type CommentRequest struct {
	Comment  string `json:"comment"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

type Comment struct {
	ID        int64      `json:"id,omitempty"`
	ParentID  *int64     `json:"parent_id,omitempty"`
	PathID    string     `json:"path_id,omitempty"`
	Path      string     `json:"-"`
	Comm      string     `json:"comm"`
	CreatedAt time.Time  `json:"created_at"`
	Children  []*Comment `json:"children,omitempty"`
}

type PaginatedComments struct {
	Comments []*Comment `json:"comments"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	Limit    int        `json:"limit"`
}
