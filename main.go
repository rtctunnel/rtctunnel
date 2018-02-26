package main

import (
	"flag"

	"github.com/gin-gonic/gin"
	"github.com/rtctunnel/rtctunnel/operator"
	"github.com/rtctunnel/rtctunnel/websocketgateway"
	"github.com/rtctunnel/rtctunnel/www"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func main() {
	var addr string

	flag.StringVar(&addr, "addr", "127.0.0.1:8000", "the address to listen on")
	flag.Parse()

	r := gin.New()
	r.Use(newZapLogger())
	defer log.Sync()

	operator.Register(log, r)
	websocketgateway.Register(log, r)
	www.Register(log, r)

	log.Info("starting http server",
		zap.String("addr", addr))
	err := r.Run(addr)
	log.Info("http server stopped")
	if err != nil {
		log.Fatal("failed to start http server", zap.Error(err))
	}
}
