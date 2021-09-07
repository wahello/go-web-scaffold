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
func NewServer(opt ServerOpt) (server *GracefulServer) {
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

type GracefulServer struct {
	server *http.Server
	logger *zap.Logger
	closed chan struct{}
}

func (s *GracefulServer) watchSignal() {
	const gracefulStopTimeout = 10 * time.Second

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	received := <-quit
	s.logger.Info("received signal, exiting...",
		zap.String("signal", received.String()),
		zap.String("addr", s.server.Addr),
	)

	defer close(s.closed)

	ctx, cancel := context.WithTimeout(context.Background(), gracefulStopTimeout)
	defer cancel()

	err := s.server.Shutdown(ctx)
	if err != nil {
		err = fmt.Errorf("server.Shutdown: %w", err)
		s.logger.Error("graceful shutdown failed.",
			zap.Error(err),
			zap.String("addr", s.server.Addr),
		)
		return
	}

	s.logger.Info("API service exited successfully.",
		zap.String("addr", s.server.Addr),
	)
}

func (s *GracefulServer) ListenAndServe() (err error) {
	err = s.server.ListenAndServe()
	if err != http.ErrServerClosed {
		err = fmt.Errorf("server stopped unexpectedly: %w", err)
		return
	}

	// ListenAndServe always returns a non-nil error.
	// After Shutdown or Close, the returned error is ErrServerClosed.
	err = nil

	<-s.closed

	return
}

// newServer returns a server with graceful shutdown
func newServer(opt ServerOpt, handler http.Handler) (server *GracefulServer) {
	server = &GracefulServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", opt.Port),
			Handler: handler,
		},
		logger: opt.Logger,
		closed: make(chan struct{}),
	}

	go server.watchSignal()

	return
}
