package controller

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"telescope/errorcode"
	"telescope/limitreader"
	"time"

	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	maxRequestBodySize = 512 * 1024
)

// RecoveryMiddleware recover from panic and log
func (con *Controller) RecoveryMiddleware(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())

			con.L.Error("panic recovered!",
				zap.Any("panic", err),
				zap.String("stack", stack),
				zap.String("method", c.Request.Method),
				zap.String("host", c.Request.Host),
				zap.String("path", c.Request.URL.Path),
				zap.String("remoteAddr", c.ClientIP()),
				zap.String("UA", c.Request.UserAgent()),
				zap.Strings("errors", c.Errors.Errors()),
			)

			if !c.Writer.Written() {
				c.PureJSON(http.StatusInternalServerError, R{
					Code: http.StatusInternalServerError,
					Msg:  http.StatusText(http.StatusInternalServerError),
					Data: nil,
				})
			}
		}
	}()

	c.Next()
}

// ErrorMiddleware deal with errors
func (con *Controller) ErrorMiddleware(c *gin.Context) {
	c.Next()

	var err = c.Errors.Last()
	if err == nil {
		return
	}

	var (
		statusCode int
		resp       R
		apiErr     *errorcode.Error
	)

	switch true {
	case err.IsType(gin.ErrorTypeBind):
		resp.Code = errorcode.CodeBadBinding
		statusCode = http.StatusNotAcceptable
	case errors.As(err.Err, &apiErr):
		resp.Code = apiErr.Code()
		statusCode = apiErr.StatusCode()
	default:
		resp.Code = errorcode.CodeGeneralError
		statusCode = http.StatusOK
	}

	resp.Msg = err.Error()
	c.PureJSON(statusCode, resp)
}

// LogMiddleware log the status of every request
func (con *Controller) LogMiddleware(c *gin.Context) {
	startedAt := time.Now()

	c.Next()

	// sometimes we just don't want log
	if c.GetBool(ctxSkipLoggingKey) {
		return
	}

	logger := con.L

	logger.Info("APIAuditLog",
		zap.String("method", c.Request.Method),
		zap.String("host", c.Request.Host),
		zap.String("origin", c.Request.Header.Get("Origin")),
		zap.String("referer", c.Request.Referer()),
		zap.String("path", c.Request.URL.Path),
		zap.String("clientIP", c.ClientIP()),
		zap.String("UA", c.Request.UserAgent()),
		zap.Int("status", c.Writer.Status()),
		zap.Duration("lapse", time.Since(startedAt)),
		zap.Int64("reqLength", c.Request.ContentLength),
		zap.Int("resLength", c.Writer.Size()),
		zap.Strings("errors", c.Errors.Errors()),
	)
}

// CORSMiddleware allows CORS request
func CORSMiddleware(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Max-Age", "43200")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Token")

	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	c.Next()
}

// LimitReaderMiddleware limits the request size
func (con *Controller) LimitReaderMiddleware(c *gin.Context) {
	if c.Request.ContentLength > maxRequestBodySize {
		_ = c.Error(fmt.Errorf("oversized payload by content length, got %d bytes, limit %d bytes", c.Request.ContentLength, maxRequestBodySize))

		c.Abort()
		return
	}

	c.Request.Body = limitreader.NewReadCloser(c.Request.Body, maxRequestBodySize)

	c.Next()
}
