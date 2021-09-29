package controller

import (
	_ "embed" // embed static page
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"telescope/cache"
	"telescope/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	ctxSkipLoggingKey   = "skipLogging"
	ctxRequestAuditKey  = "requestAudit"
	ctxResponseAuditKey = "responseAudit"
)

// Controller is where http logic lives
type Controller struct {
	L             *zap.Logger
	D             *database.DB
	Red           *cache.Cache
	AuditResponse bool
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
	var dirPath string
	if usePwd, _ := strconv.ParseBool(os.Getenv("WEB_ROOT_USE_PWD")); usePwd {
		dirPath = root
	} else {
		exePath, err := os.Executable()
		if err != nil {
			err = fmt.Errorf("os.Executable: %w", err)
			panic(err)
		}
		dirPath = path.Join(path.Dir(exePath), root)
	}

	staticHandler := http.FileServer(http.FS(&fallbackIndexPageFS{
		inner: os.DirFS(dirPath),
	}))

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

type fallbackIndexPageFS struct {
	inner fs.FS
}

func (s *fallbackIndexPageFS) Open(name string) (f fs.File, err error) {
	const indexPage = "index.html"

	f, err = s.inner.Open(name)
	if err == nil {
		return
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return
	}

	if name == indexPage || path.Ext(name) != "" {
		return
	}

	f, err = s.inner.Open(indexPage)
	return
}

//go:embed telescope-index.html
var indexPageContent []byte

func (con *Controller) IndexPage(c *gin.Context) {
	skipLogging(c)
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusNoContent)
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", indexPageContent)
}
