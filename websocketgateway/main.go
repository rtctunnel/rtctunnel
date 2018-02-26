package websocketgateway

import (
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Register registers the http handlers for the websocket gateway
func Register(log *zap.Logger, r gin.IRouter) {
	g := r.Group("websocketgateway")
	g.GET("/dial/:addr", func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		defer ws.Close()

		addr := c.Param("addr")
		err = wsDial(ws, addr)
		if err != nil {
			ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(500, err.Error()), time.Now().Add(time.Second*3))
			c.AbortWithError(500, err)
		}
	})
	g.GET("/listen/:addr", func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		defer ws.Close()

		addr := c.Param("addr")
		err = wsListen(log, ws, addr)
		if err != nil {
			ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(500, err.Error()), time.Now().Add(time.Second*3))
			c.AbortWithError(500, err)
		}
	})
	g.GET("/", func(c *gin.Context) {
		c.String(200, "OK")
	})
}

func wsDial(ws *websocket.Conn, addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	wsrwc := &wsBinaryReadWriteCloser{Conn: ws}
	err = proxy(conn, wsrwc)
	if err != nil {
		return err
	}

	return nil
}

func wsListen(log *zap.Logger, ws *websocket.Conn, addr string) error {
	wsrwc := &wsBinaryReadWriteCloser{Conn: ws}

	dst, err := yamux.Client(wsrwc, yamux.DefaultConfig())
	if err != nil {
		return errors.Wrap(err, "failed to establish yamux session")
	}
	defer dst.Close()

	src, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrapf(err, "failed to listen on: %s", addr)
	}
	defer src.Close()

	// if the "server" disconnects, close the listener too
	go func() {
		for range time.Tick(time.Second) {
			if dst.IsClosed() {
				src.Close()
				return
			}
		}
	}()

	log.Info("started tcp listener",
		zap.String("addr", src.Addr().String()),
		zap.String("destination", dst.Addr().String()))
	defer log.Info("stopped tcp listener",
		zap.String("addr", src.Addr().String()),
		zap.String("destination", dst.Addr().String()))

	for {
		srcc, err := src.Accept()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				time.Sleep(time.Second)
				continue
			}
			return errors.Wrap(err, "failed to accept a new TCP connection")
		}

		dstc, err := dst.Open()
		if err != nil {
			srcc.Close()
			return errors.Wrap(err, "failed to open connection to destination")
		}

		go func() {
			defer srcc.Close()
			defer dstc.Close()
			proxy(dstc, srcc)
		}()
	}
}

func proxy(dst, src io.ReadWriter) error {
	var eg errgroup.Group
	eg.Go(func() error {
		_, err := io.Copy(dst, src)
		return err
	})
	eg.Go(func() error {
		_, err := io.Copy(src, dst)
		return err
	})
	return eg.Wait()
}
