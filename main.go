package main

import (
	"context"
	"log"

	"ai-knowledge-assistant/Repository"
	"ai-knowledge-assistant/config"
	"ai-knowledge-assistant/database"
	"ai-knowledge-assistant/handler"
	"ai-knowledge-assistant/routes"
	"ai-knowledge-assistant/service"
	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		log.Fatal(err)
	}
	repo := repository.Repository{DB: db}
	h := handler.Handler{Repo: repo, DocumentService: service.DocumentService{Repo: repo, StoragePath: cfg.StoragePath}, Ask: service.AskService{Repo: repo, LLM: service.LLM{Config: cfg}, Config: cfg}, Config: cfg}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	routes.Register(r, h, cfg)
	log.Printf("server listening on :%s", cfg.Port)
	log.Fatal(r.Run(":" + cfg.Port))
}
