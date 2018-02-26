package websocketgateway

import (
	"io"

	"github.com/gorilla/websocket"
)

type wsBinaryReadWriteCloser struct {
	*websocket.Conn
	readbuffer []byte
}

var _ io.ReadWriteCloser = (*wsBinaryReadWriteCloser)(nil)

func (conn *wsBinaryReadWriteCloser) Read(p []byte) (int, error) {
	for {
		switch {
		case len(conn.readbuffer) > len(p):
			copy(p, conn.readbuffer)
			sz := len(p)
			conn.readbuffer = conn.readbuffer[sz:]
			return sz, nil
		case len(conn.readbuffer) > 0 && len(conn.readbuffer) < len(p):
			copy(p, conn.readbuffer)
			sz := len(conn.readbuffer)
			conn.readbuffer = nil
			return sz, nil
		}

		mt, buf, err := conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if mt != websocket.BinaryMessage {
			continue
		}

		conn.readbuffer = append(conn.readbuffer, buf...)
	}
}

func (conn *wsBinaryReadWriteCloser) Write(p []byte) (int, error) {
	err := conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
