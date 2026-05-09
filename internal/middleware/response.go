package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/kserve-nexus/pkg/log"
)

const (
	SUCCESS = 0
	FAIL    = 1
)

// Response 返回通用信息
// @Description 返回通用信息
type Response struct {
	Code    int         `json:"code"`    // 状态码
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 数据内容
}

// SuccessJson 成功返回结构体
func SuccessJson(c *gin.Context, data any) {
	c.JSON(200, gin.H{"code": SUCCESS, "message": "请求成功", "data": data})
}

// ErrorJson 异常返回结构体
func ErrorJson(c *gin.Context, err error, msg string) {
	log.Logger.Error(err, msg)
	c.JSON(200, gin.H{"code": FAIL, "message": msg})
}

// ResponseJson 通用返回结构体
func ResponseJson(c *gin.Context, data any, err error) {
	status := SUCCESS
	if err != nil {
		status = FAIL
		log.Logger.Error(err, "服务异常")
	}
	c.JSON(200, gin.H{"code": status, "message": "请求错误", "data": data})
}
