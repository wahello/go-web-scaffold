package controller

import (
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"telescope/errorcode"
	"time"

	"github.com/valyala/bytebufferpool"

	"github.com/nanmu42/limitio"

	"github.com/signalsciences/ac/acascii"

	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	maxRequestBodySize = 256 << 10
)

// RecoveryMiddleware recover from panic and log
func (con *Controller) RecoveryMiddleware(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())

			con.Logger.Error("panic recovered!",
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
	// abort if there's already a response body
	if c.Writer.Written() {
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

	latency := time.Since(startedAt)

	logger := con.Logger
	if reqBody, ok := c.Get(ctxRequestAuditKey); ok {
		logger = logger.With(zap.Stringp("requestBody", reqBody.(*string)))
	}
	if respBody, ok := c.Get(ctxResponseAuditKey); ok {
		logger = logger.With(zap.Stringp("responseBody", respBody.(*string)))
	}

	logger.Info("APIAuditLog",
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

// CORSMiddleware allows CORS request
func CORSMiddleware(c *gin.Context) {
	header := c.Writer.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Max-Age", "43200")
	header.Set("Access-Control-Allow-Methods", "POST")
	header.Set("Access-Control-Allow-Headers", "Content-Type, Token")

	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	c.Next()
}

// LimitReaderMiddleware limits the request size
func (con *Controller) LimitReaderMiddleware(maxRequestBodyBytes int) func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.Request.ContentLength > int64(maxRequestBodyBytes) {
			_ = c.Error(fmt.Errorf("oversized payload by content length, got %d bytes, limit %d bytes", c.Request.ContentLength, maxRequestBodyBytes))

			c.Abort()
			return
		}

		c.Request.Body = limitio.NewReadCloser(c.Request.Body, maxRequestBodyBytes, false)

		c.Next()
	}
}

// PayloadAuditLogMiddleware audits text request and response then logs them.
// This middleware replies on LogMiddleware to output,
// use it inside LogMiddleware so that LogMiddleware can see this one's product.
//
// Note:
//
// If there's a gzip middleware, use it outside this one so this one can see response before compressing.
func (con *Controller) PayloadAuditLogMiddleware() func(c *gin.Context) {
	const (
		RequestBodyMaxLength  = 512
		ResponseBodyMaxLength = 512
	)

	var textPayloadMIME = []string{
		"application/json", "text/xml", "application/xml", "text/html",
		"text/richtext", "text/plain", "text/css", "text/x-script",
		"text/x-component", "text/x-markdown", "application/javascript",
	}
	MIMEChecker := acascii.MustCompileString(textPayloadMIME)

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead ||
			c.Request.Method == http.MethodOptions {
			return
		}

		var reqBuf = bytebufferpool.Get()
		defer bytebufferpool.Put(reqBuf)
		limitedReqBuf := limitio.NewWriter(reqBuf, RequestBodyMaxLength, true)

		c.Request.Body = &readCloser{
			Reader: io.TeeReader(c.Request.Body, limitedReqBuf),
			Closer: c.Request.Body,
		}

		var respBuf *bytebufferpool.ByteBuffer
		if con.AuditResponse {
			respBuf = bytebufferpool.Get()
			defer bytebufferpool.Put(respBuf)
			limitedRespBuf := limitio.NewWriter(respBuf, ResponseBodyMaxLength, true)

			c.Writer = &logWriter{
				ResponseWriter: c.Writer,
				SavedBody:      limitedRespBuf,
			}
		}

		c.Next()

		// sometimes we just don't want log
		if c.GetBool(ctxSkipLoggingKey) {
			return
		}

		var (
			reqBody  string
			respBody string
		)

		if reqBuf.Len() > 0 {
			reqContentType := c.Request.Header.Get("Content-Type")
			if reqContentType == "" {
				reqContentType = http.DetectContentType(reqBuf.Bytes())
			}
			if reqContentType != "" && MIMEChecker.MatchString(reqContentType) {
				reqBody = reqBuf.String()
			} else {
				reqBody = "unsupported content type: " + reqContentType
			}
			c.Set(ctxRequestAuditKey, &reqBody)
		}

		//goland:noinspection ALL
		if con.AuditResponse && respBuf.Len() > 0 {
			if respContentType := c.Writer.Header().Get("Content-Type"); respContentType != "" && MIMEChecker.MatchString(respContentType) {
				//goland:noinspection ALL
				respBody = respBuf.String()
			} else {
				respBody = "unsupported content type: " + respContentType
			}

			c.Set(ctxResponseAuditKey, &respBody)
		}
	}
}

type readCloser struct {
	io.Reader
	io.Closer
}

type logWriter struct {
	gin.ResponseWriter
	SavedBody io.Writer
}

func (w *logWriter) Write(b []byte) (int, error) {
	_, _ = w.SavedBody.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *logWriter) WriteString(s string) (int, error) {
	_, _ = w.SavedBody.Write([]byte(s))
	return w.ResponseWriter.WriteString(s)
}
