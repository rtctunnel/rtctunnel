package www

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Register registers the http handlers
func Register(log *zap.Logger, r gin.IRoutes) {
	r.GET("/", func(c *gin.Context) {
		c.String(200, "Hello World")
	})
}
