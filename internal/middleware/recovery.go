package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/kserve-nexus/pkg/log"
)

// Recovery 异常处理中间件，确保正确处理异常，避免乱码返回
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录 Panic
				log.Logger.Error(nil, "请求错误: %v", r)

				// 返回统一的错误响应
				c.JSON(200, gin.H{"code": FAIL, "message": "请求错误"})
				// 中断请求
				c.Abort()
			}
		}()
		c.Next()
	}
}
