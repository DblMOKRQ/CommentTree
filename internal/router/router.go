package router

import (
	"CommentTree/internal/router/handlers"
	"CommentTree/internal/router/middleware"
	"github.com/wb-go/wbf/ginext"
	"go.uber.org/zap"
)

type Router struct {
	rout    *ginext.Engine
	handler *handlers.CommentHandler
	log     *zap.Logger
}

func NewRouter(mode string, handler *handlers.CommentHandler, log *zap.Logger) *Router {
	router := Router{
		rout:    ginext.New(mode),
		handler: handler,
		log:     log.Named("router"),
	}
	router.setupRouter()
	return &router
}

func (r *Router) setupRouter() {
	r.rout.Use(middleware.LoggingMiddleware(r.log))
	r.rout.POST("/comments", r.handler.CreateComment)
	r.rout.GET("/comments", r.handler.GetCommentByID)
	r.rout.DELETE("/comments/:id", r.handler.DeleteComment)
	r.rout.GET("/search", r.handler.SearchComments)

	r.rout.GET("/", func(c *ginext.Context) {
		c.File("./static/index.html")
	})
}

func (r *Router) GetEngine() *ginext.Engine {
	return r.rout
}

func (r *Router) Start(addr string) error {
	return r.rout.Run(addr)
}
