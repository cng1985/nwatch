package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cng1985/nwatch/erp/backend/internal/db"
	"github.com/cng1985/nwatch/erp/backend/internal/handlers"
	"github.com/cng1985/nwatch/erp/backend/internal/seed"
	"github.com/gin-gonic/gin"
)

func main() {
	addr := flag.String("addr", ":8088", "listen address")
	dbPath := flag.String("db", "", "sqlite database path (default: erp/backend/data/erp.db)")
	flag.Parse()

	path := *dbPath
	if path == "" {
		path = defaultDBPath()
	}

	gdb, err := db.Open(path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := seed.Run(gdb); err != nil {
		log.Fatalf("seed: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), corsMiddleware(), requestLog())

	api := &handlers.API{DB: gdb}
	api.Register(r)

	log.Printf("ERP demo API listening on %s (db=%s)", *addr, path)
	if err := r.Run(*addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func defaultDBPath() string {
	if v := os.Getenv("ERP_DB"); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return "data/erp.db"
	}
	return filepath.Join(wd, "data", "erp.db")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func requestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		fmt.Printf("%s %s %d %s\n", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
	}
}
