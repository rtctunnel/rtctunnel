package peer

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Dispatcher struct {
	conn *Conn

	mu        sync.Mutex
	listeners map[int]*dispatchListener
}

func NewDispatcher(conn *Conn) *Dispatcher {
	d := &Dispatcher{
		conn:      conn,
		listeners: make(map[int]*dispatchListener),
	}
	go func() {
		for {
			conn, port, err := conn.Accept()
			if err != nil {
				if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
					time.Sleep(time.Second)
					continue
				}
			}

			shouldClose := false

			d.mu.Lock()
			li, ok := d.listeners[port]
			if ok {
				select {
				case li.pending <- conn:
				default:
					shouldClose = true
				}
			} else {
				shouldClose = true
			}
			d.mu.Unlock()

			if shouldClose {
				log.Warn().Int("port", port).Msg("closing connection because no listener accepted it")
				conn.Close()
			}
		}
	}()
	return d
}

func (d *Dispatcher) Listen(port int) net.Listener {
	d.mu.Lock()
	defer d.mu.Unlock()
	dl := newDispatchListener(d, port)
	d.listeners[port] = dl
	return dl
}

type dispatchListener struct {
	dispatcher *Dispatcher
	port       int
	pending    chan net.Conn
	closer     chan struct{}
}

func newDispatchListener(dispatcher *Dispatcher, port int) *dispatchListener {
	return &dispatchListener{
		dispatcher: dispatcher,
		port:       port,
		pending:    make(chan net.Conn, 16),
		closer:     make(chan struct{}),
	}
}

func (dl *dispatchListener) Accept() (net.Conn, error) {
	select {
	case conn := <-dl.pending:
		return conn, nil
	case <-dl.closer:
		return nil, errors.New("listener closed")
	}
}

func (dl *dispatchListener) Addr() net.Addr {
	return dispatchListenerAddr{dl.port}
}

func (dl *dispatchListener) Close() error {
	dl.dispatcher.mu.Lock()
	delete(dl.dispatcher.listeners, dl.port)
	dl.dispatcher.mu.Unlock()
	return nil
}

type dispatchListenerAddr struct {
	port int
}

func (addr dispatchListenerAddr) Network() string {
	return "webrtc:dispatcher"
}

func (addr dispatchListenerAddr) String() string {
	return fmt.Sprintf("%d", addr.port)
}
