package handlers

import (
	"CommentTree/internal/models"
	"CommentTree/internal/service"
	"encoding/json"
	"github.com/wb-go/wbf/ginext"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

type CommentHandler struct {
	service service.Service
}

func NewCommentHandler(service service.Service) *CommentHandler {
	return &CommentHandler{service: service}
}

func (h *CommentHandler) CreateComment(c *ginext.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	log.Debug("Creating comment")
	commentRequest := &models.CommentRequest{}
	if err := json.NewDecoder(c.Request.Body).Decode(commentRequest); err != nil {
		log.Error("Failed to decode request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, ginext.H{"error": "Invalid request body"})
		return
	}
	if commentRequest.Comment == "" {
		log.Warn("Comment is required")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "Comment is required"})
		return
	}
	if commentRequest.ParentID != nil && *commentRequest.ParentID < 0 {
		log.Warn("ParentID It can't be negative")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "ParentID it can't be negative"})
	}

	err := h.service.CreateComment(c.Request.Context(), *commentRequest)
	if err != nil {
		log.Error("Failed to create comment", zap.Error(err))
		c.JSON(http.StatusBadRequest, ginext.H{"error": "Failed to create comment"})
		return
	}
	log.Debug("Created comment")
	c.JSON(http.StatusCreated, ginext.H{"comment": commentRequest})
}
func (h *CommentHandler) GetCommentByID(c *ginext.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	idStr := c.Query("parent")

	if idStr == "" {
		log.Debug("Parent ID not provided, fetching all top-level comments")

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		sortBy := c.DefaultQuery("sort_by", "created_at")
		sortOrder := c.DefaultQuery("sort_order", "asc")

		paginatedResult, err := h.service.GetAllCommentTrees(c.Request.Context(), page, limit, sortBy, sortOrder)
		if err != nil {
			log.Error("Failed to get all comment trees", zap.Error(err))
			c.JSON(http.StatusInternalServerError, ginext.H{"error": "Failed to get all comments"})
			return
		}
		c.JSON(http.StatusOK, paginatedResult)
		return
	}

	log.Debug("Getting comment by ID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Error("Failed to parse parent ID", zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, ginext.H{"error": "Invalid id format"})
		return
	}

	comments, err := h.service.GetComments(c.Request.Context(), id)
	if err != nil {
		log.Error("Failed to get comments", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "Failed to get comments"})
		return
	}
	c.JSON(http.StatusOK, ginext.H{"comments": comments})
}
func (h *CommentHandler) DeleteComment(c *ginext.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	log.Debug("Deleting comment")
	idStr := c.Param("id")
	if idStr == "" {
		log.Error("Failed to get comment by ID", zap.String("id", idStr))
		c.JSON(http.StatusBadRequest, ginext.H{"error": "Invalid id"})
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Error("Failed to get comment by ID", zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, ginext.H{"error": "Invalid id"})
		return
	}
	err = h.service.DeleteComments(c.Request.Context(), id)
	if err != nil {
		log.Error("Failed to delete comments", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "Failed to delete comments"})
		return
	}
	log.Debug("Deleted comment")
	c.JSON(http.StatusOK, ginext.H{"id": id})
}

func (h *CommentHandler) SearchComments(c *ginext.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	query := c.Query("q")

	log.Debug("Searching for comments", zap.String("query", query))
	results, err := h.service.SearchComments(c.Request.Context(), query)
	if err != nil {
		log.Error("Failed to search comments", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "Failed to perform search"})
		return
	}

	c.JSON(http.StatusOK, ginext.H{"comments": results})
}
