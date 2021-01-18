package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"telescope/errorcode"
	"telescope/limitreader"
	"time"

	"github.com/signalsciences/ac/acascii"

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

// PayloadAuditMiddleware audits text request and response then logs them.
//
// Note:
//
// * If there's a gzip middleware, use it outside this one so this one can see response before compressing.
//
// * If there's a LimitReaderMiddleware, use it outside this one so this one is also protected.
func (con *Controller) PayloadAuditLogMiddleware(auditResponse bool) func(c *gin.Context) {
	var textPayloadMIME = []string{"application/json", "text/xml", "application/xml", "text/html", "text/richtext", "text/plain", "text/css", "text/x-script", "text/x-component", "text/x-markdown", "application/javascript"}
	MIMEChecker := acascii.MustCompileString(textPayloadMIME)

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			return
		}

		var reqBuf bytes.Buffer

		c.Request.Body = &readCloser{
			reader: io.TeeReader(c.Request.Body, &reqBuf),
			closer: c.Request.Body,
		}

		var respBuf *bytes.Buffer
		if auditResponse {
			respBuf = &bytes.Buffer{}
			c.Writer = &logWriter{
				ResponseWriter: c.Writer,
				SavedBody:      respBuf,
			}
		}

		startedAt := time.Now()

		c.Next()

		// sometimes we just don't want log
		if c.GetBool(ctxSkipLoggingKey) {
			return
		}

		latency := time.Since(startedAt)

		L := con.L

		reqContentType := c.Request.Header.Get("Content-Type")
		if reqContentType == "" && reqBuf.Len() > 0 {
			reqContentType = http.DetectContentType(reqBuf.Bytes())
		}
		if reqContentType != "" && MIMEChecker.MatchString(reqContentType) {
			L = L.With(zap.String("requestBody", reqBuf.String()))
		} else {
			L = L.With(zap.String("requestBody", "unsupported content type: "+reqContentType))
		}
		if auditResponse {
			if respContentType := c.Writer.Header().Get("Content-Type"); respContentType != "" && MIMEChecker.MatchString(respContentType) {
				//goland:noinspection ALL
				L = L.With(zap.String("responseBody", respBuf.String()))
			} else {
				L = L.With(zap.String("responseBody", "unsupported content type: "+respContentType))
			}
		}

		L.Info("PayloadAuditLog",
			zap.String("method", c.Request.Method),
			zap.String("host", c.Request.Host),
			zap.String("origin", c.Request.Header.Get("Origin")),
			zap.String("referer", c.Request.Referer()),
			zap.String("path", c.Request.URL.Path),
			zap.String("clientIP", c.ClientIP()),
			zap.String("UA", c.Request.UserAgent()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("lapse", latency),
			zap.Int64("reqLength", c.Request.ContentLength),
			zap.Int("resLength", c.Writer.Size()),
			zap.Strings("errors", c.Errors.Errors()),
		)
	}
}

type readCloser struct {
	reader io.Reader
	closer io.Closer
}

func (r *readCloser) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *readCloser) Close() error {
	return r.closer.Close()
}

type logWriter struct {
	gin.ResponseWriter
	SavedBody *bytes.Buffer
}

func (w *logWriter) Write(b []byte) (int, error) {
	_, _ = w.SavedBody.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *logWriter) WriteString(s string) (int, error) {
	_, _ = w.SavedBody.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
