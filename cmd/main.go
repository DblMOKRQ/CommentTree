package main

import (
	"CommentTree/internal/repository"
	"CommentTree/internal/router"
	"CommentTree/internal/router/handlers"
	"CommentTree/internal/service"
	"CommentTree/pkg/logger"
	"errors"
	"github.com/wb-go/wbf/config"
	"go.uber.org/zap"
	"net/http"
)

func main() {
	cfg := config.New()
	_ = cfg.LoadConfigFiles("./config/config.yaml")
	log, err := logger.NewLogger(cfg.GetString("log_level"))
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	repo, err := repository.NewRepository(cfg.GetString("master_dsn"), cfg.GetStringSlice("slaveDSNs"), log)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	serviceComment := service.NewService(repo, log)
	handlersComment := handlers.NewCommentHandler(*serviceComment)
	rout := router.NewRouter(cfg.GetString("log_level"), handlersComment, log)
	srv := &http.Server{
		Addr:    cfg.GetString("addr"),
		Handler: rout.GetEngine(),
	}
	log.Info("Starting server", zap.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("Failed to listen and server", zap.Error(err))
	}
}
