package controller

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// R is the response envelope
type R struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func ok(c *gin.Context, data interface{}) {
	c.PureJSON(http.StatusOK, R{
		Code: 0,
		Msg:  "OK",
		Data: data,
	})
}

// secureToken provide crypto-safe and url-safe random string
func secureToken(minLength int) (randomStr string, err error) {
	buf := make([]byte, minLength*3/4)
	_, err = rand.Read(buf)
	if err != nil {
		err = fmt.Errorf("rand.Read: %w", err)
		return
	}

	randomStr = base64.URLEncoding.EncodeToString(buf)
	return
}
