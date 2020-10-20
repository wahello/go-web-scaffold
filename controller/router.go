package controller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"telescope/cache"
	"telescope/database"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nanmu42/gzip"
	"go.uber.org/zap"
)

// ServerOpt options to start a new server
type ServerOpt struct {
	Port     int
	Logger   *zap.Logger
	Database *database.DB
	Redis    *cache.Red
}

// NewServer fires a new server
func NewServer(opt ServerOpt) (server *http.Server) {
	control := &Controller{
		L:   opt.Logger.With(zap.String("side", "public")),
		D:   opt.Database,
		Red: opt.Redis,
	}
	handler := newGin(control)
	handler.Use(CORSMiddleware)

	// register API route

	// robots.txt
	handler.GET("/robots.txt", control.RobotsTXT)

	// health
	group := handler.Group("/api")
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
		con.LogMiddleware,
		con.ErrorMiddleware,
		con.LimitReaderMiddleware,
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
		// kill (no param) default send syscall.SIGTERM
		// kill (no param) default send syscall.SIGTERM
		// kill -2 is syscall.SIGINT
		// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
		//
		// It is allowed to call Notify multiple times with different channels and the same signals:
		// each channel receives copies of incoming signals independently.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
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

// serveFileWhenNotFound tries to serve static file when there is no
// matched route to API.
func serveFileWhenNotFound(root string) func(c *gin.Context) {
	staticHandler := http.FileServer(gin.Dir(root, false))

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet ||
			c.Request.Method == http.MethodHead {
			skipLogging(c)
			staticHandler.ServeHTTP(c.Writer, c.Request)
			return
		}

		c.PureJSON(http.StatusNotFound, R{
			Code: http.StatusNotFound,
			Msg:  http.StatusText(http.StatusNotFound),
			Data: nil,
		})
	}
}
