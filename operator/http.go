package operator

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Register registers the routes with the given router group
func Register(log *zap.Logger, r gin.IRouter) {
	g := r.Group("operator")
	g.GET("/recv/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.String(200, "OK", id)
	})

	g.GET("/send/:id", func(c *gin.Context) {
		id := c.Param("id")
		msg := c.Query("msg")
		c.String(200, "OK", id, msg)
	})

	g.GET("/", func(c *gin.Context) {
		c.String(200, "RTCTunnel Signalman")
	})
}
