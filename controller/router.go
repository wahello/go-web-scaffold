package controller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"telescope/cache"
	"telescope/database"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nanmu42/gzip"
	"go.uber.org/zap"
)

// ServerOpt options to start a new server
type ServerOpt struct {
	Port          int
	Logger        *zap.Logger
	Database      *database.DB
	Redis         *cache.Red
	AuditResponse bool
}

// NewServer fires a new server
func NewServer(opt ServerOpt) (server *http.Server) {
	control := &Controller{
		L:             opt.Logger,
		D:             opt.Database,
		Red:           opt.Redis,
		AuditResponse: opt.AuditResponse,
	}
	handler := newGin(control)

	// register API route

	// index page
	handler.HEAD("/", control.IndexPage)
	handler.GET("/", control.IndexPage)

	// robots.txt
	handler.HEAD("/robots.txt", control.RobotsTXT)
	handler.GET("/robots.txt", control.RobotsTXT)

	// health
	group := handler.Group("/api")
	group.HEAD("/hello", control.Hello)
	group.GET("/hello", control.Hello)

	server = newServer(opt, handler)
	return
}

// newGin get you a glass of gin, flavored
func newGin(con *Controller) (g *gin.Engine) {
	g = gin.New()

	g.ForwardedByClientIP = true
	g.HandleMethodNotAllowed = true
	g.NoMethod(con.MethodNotAllowed)
	g.NoRoute(con.NotFound)

	g.Use(
		con.RecoveryMiddleware,
		gzip.DefaultHandler().Gin,
		con.LimitReaderMiddleware(maxRequestBodySize),
		con.LogMiddleware,
		con.PayloadAuditLogMiddleware(),
		con.ErrorMiddleware,
	)

	return
}

// newServer returns a server with graceful shutdown
func newServer(opt ServerOpt, handler http.Handler) (server *http.Server) {
	server = &http.Server{
		Addr:    fmt.Sprintf(":%d", opt.Port),
		Handler: handler,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		received := <-quit
		opt.Logger.Info("received signal, exiting...",
			zap.String("signal", received.String()),
			zap.Int("port", opt.Port),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		shutdownErr := server.Shutdown(ctx)
		if shutdownErr != nil {
			shutdownErr = fmt.Errorf("server.Shutdown: %w", shutdownErr)
			opt.Logger.Error("graceful shutdown failed.",
				zap.Error(shutdownErr),
				zap.Int("port", opt.Port),
			)
			return
		}

		opt.Logger.Info("API service exits successfully.",
			zap.Int("port", opt.Port),
		)
	}()

	return
}
