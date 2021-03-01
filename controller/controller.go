package controller

import (
	"net/http"
	"telescope/cache"
	"telescope/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const ctxSkipLoggingKey = "skipLogging"

// Controller is where http logic lives
type Controller struct {
	L   *zap.Logger
	D   *database.DB
	Red *cache.Red
}

// skipLogging marks when we don't want logging
func skipLogging(c *gin.Context) {
	c.Set(ctxSkipLoggingKey, true)
}

// Hello says hello world,
// also stands for health check.
func (con *Controller) Hello(c *gin.Context) {
	skipLogging(c)
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusNoContent)
		return
	}
	ok(c, "hello world!")
}

// NotFound 404 not found handler
func (con *Controller) NotFound(c *gin.Context) {
	c.PureJSON(http.StatusNotFound, R{
		Code: http.StatusNotFound,
		Msg:  http.StatusText(http.StatusNotFound),
		Data: nil,
	})
}

// MethodNotAllowed 405 method not allowed handler
func (con *Controller) MethodNotAllowed(c *gin.Context) {
	c.PureJSON(http.StatusMethodNotAllowed, R{
		Code: http.StatusMethodNotAllowed,
		Msg:  http.StatusText(http.StatusMethodNotAllowed),
		Data: nil,
	})
}

func (con *Controller) RobotsTXT(c *gin.Context) {
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusNoContent)
		return
	}
	c.String(http.StatusOK, `User-agent: *
Disallow: /`)
}

// ServeFileWhenNotFound tries to serve static file when there is no
// matched route to API.
func ServeFileWhenNotFound(root string) func(c *gin.Context) {
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
