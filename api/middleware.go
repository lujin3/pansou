package api

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	}
}

// LoggerMiddleware 日志中间件
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		startTime := time.Now()
		
		// 处理请求
		c.Next()
		
		// 结束时间
		endTime := time.Now()
		
		// 执行时间
		latencyTime := endTime.Sub(startTime)
		
		// 请求方式
		reqMethod := c.Request.Method
		
		// 请求路由
		reqURI := c.Request.RequestURI
		
		// 对于搜索API，尝试解码关键词以便更好地显示
		displayURI := reqURI
		if strings.Contains(reqURI, "/api/search") && strings.Contains(reqURI, "kw=") {
			if parsedURL, err := url.Parse(reqURI); err == nil {
				if keyword := parsedURL.Query().Get("kw"); keyword != "" {
					if decodedKeyword, err := url.QueryUnescape(keyword); err == nil {
						// 替换原始URI中的编码关键词为解码后的关键词
						displayURI = strings.Replace(reqURI, "kw="+keyword, "kw="+decodedKeyword, 1)
					}
				}
			}
		}
		
		// 状态码
		statusCode := c.Writer.Status()
		
		// 请求IP
		clientIP := c.ClientIP()
		
		// 日志格式
		gin.DefaultWriter.Write([]byte(
			fmt.Sprintf("| %s | %s | %s | %d | %s\n", 
				clientIP, reqMethod, displayURI, statusCode, latencyTime.String())))
	}
} 



func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 使用 Authorization: Bearer <TOKEN> 进行鉴权
        token := c.GetHeader("Authorization")
        expected := os.Getenv("TOKEN")
        if expected == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "TOKEN not configured"})
            c.Abort()
            return
        }
        if token != fmt.Sprintf("Bearer %s", expected) {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            c.Abort()
            return
        }
        c.Next()
    }
}
	
