package service

import (
	"CommentTree/internal/models"
	"context"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"strings"
	"time"
)

type Repository interface {
	Create(ctx context.Context, c models.Comment) error
	GetByID(ctx context.Context, id int64) (*models.Comment, error)
	GetChildrenByPath(ctx context.Context, path string) ([]*models.Comment, error)
	DeleteByPath(ctx context.Context, path string) error
	GetTopLevelComments(ctx context.Context, limit, offset int, sortBy, sortOrder string) ([]*models.Comment, int, error)
	SearchByText(ctx context.Context, query string) ([]*models.Comment, error)
}

type Service struct {
	repo Repository
	log  *zap.Logger
}

func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{
		repo: repo,
		log:  log.Named("service"),
	}
}

func (s *Service) CreateComment(ctx context.Context, cr models.CommentRequest) error {
	pathID := uuid.New().String()
	comment := models.Comment{
		ParentID:  cr.ParentID,
		PathID:    pathID,
		Comm:      cr.Comment,
		CreatedAt: time.Now(),
	}
	err := s.repo.Create(ctx, comment)
	if err != nil {
		s.log.Error("Failed to create comment", zap.Error(err))
		return fmt.Errorf("failed to create comment: %w", err)
	}
	return nil
}

func (s *Service) GetComments(ctx context.Context, id int64) (*models.Comment, error) {
	s.log.Debug("Getting comments tree starting from id", zap.Int64("id", id))

	root, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.log.Error("Failed to get root comment", zap.Error(err))
		return nil, fmt.Errorf("failed to get root comment: %w", err)
	}

	allComments, err := s.repo.GetChildrenByPath(ctx, root.Path)
	if err != nil {
		s.log.Error("Failed to get children", zap.Error(err))
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	s.log.Debug("Got all comments for the subtree", zap.Int("count", len(allComments)))

	if len(allComments) <= 1 {
		root.Children = []*models.Comment{}
		return root, nil
	}

	commentMap := make(map[int64]*models.Comment)
	for _, comment := range allComments {
		comment.Children = []*models.Comment{}
		commentMap[comment.ID] = comment
	}

	for _, comment := range allComments {
		if comment.ParentID != nil {
			if parent, ok := commentMap[*comment.ParentID]; ok {
				parent.Children = append(parent.Children, comment)
			}
		}
	}

	if rootInTree, ok := commentMap[id]; ok {
		return rootInTree, nil
	}

	return root, nil
}

func (s *Service) DeleteComments(ctx context.Context, id int64) error {
	s.log.Debug("Deleting root comment", zap.Int64("id", id))
	rootComment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.log.Error("Failed to get root comment", zap.Error(err))
		return fmt.Errorf("failed to get root comment: %w", err)
	}
	err = s.repo.DeleteByPath(ctx, rootComment.Path)
	if err != nil {
		s.log.Error("Failed to delete comments", zap.Error(err), zap.String("path", rootComment.Path))
		return fmt.Errorf("failed to delete comments: %w", err)
	}
	return nil
}

func (s *Service) GetAllCommentTrees(ctx context.Context, page, limit int, sortBy, sortOrder string) (*models.PaginatedComments, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10 // Значение по умолчанию
	}
	offset := (page - 1) * limit

	s.log.Debug("Getting paginated top-level comments", zap.Int("page", page), zap.Int("limit", limit))
	topLevelComments, total, err := s.repo.GetTopLevelComments(ctx, limit, offset, sortBy, sortOrder)
	if err != nil {
		s.log.Error("Failed to get top-level comments", zap.Error(err))
		return nil, fmt.Errorf("failed to get top-level comments: %w", err)
	}

	var allTrees []*models.Comment
	for _, rootComment := range topLevelComments {
		fullTree, err := s.GetComments(ctx, rootComment.ID)
		if err != nil {
			s.log.Error("Failed to build tree for root comment", zap.Int64("id", rootComment.ID), zap.Error(err))
			continue
		}
		allTrees = append(allTrees, fullTree)
	}

	return &models.PaginatedComments{
		Comments: allTrees,
		Total:    total,
		Page:     page,
		Limit:    limit,
	}, nil
}

func (s *Service) SearchComments(ctx context.Context, query string) ([]*models.Comment, error) {
	s.log.Debug("Searching for comments", zap.String("query", query))
	if len(strings.TrimSpace(query)) < 3 {
		return []*models.Comment{}, nil
	}
	return s.repo.SearchByText(ctx, query)
}
