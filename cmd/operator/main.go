package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
)

const (
	maxMessageSize = 10 * 1024
)

func main() {
	log.SetHandler(text.New(os.Stderr))

	o := newOperator()

	http.HandleFunc("/pub", func(w http.ResponseWriter, r *http.Request) {
		addr := r.FormValue("address")
		data := r.FormValue("data")
		if len(data) > maxMessageSize {
			log.WithField("remote-addr", r.RemoteAddr).
				WithField("addr", addr).
				Warn("data too large")
			http.Error(w, "data too large", http.StatusBadRequest)
			return
		}

		log.WithField("remote-addr", r.RemoteAddr).
			WithField("addr", addr).
			WithField("data", data).
			Info("pub")

		ctx, cancel := context.WithTimeout(r.Context(), time.Second*30)
		defer cancel()

		err := o.Pub(ctx, addr, data)
		if err == nil {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	})
	http.HandleFunc("/sub", func(w http.ResponseWriter, r *http.Request) {
		addr := r.FormValue("address")

		log.WithField("remote-addr", r.RemoteAddr).
			WithField("addr", addr).
			Info("sub")

		ctx, cancel := context.WithTimeout(r.Context(), time.Second*30)
		defer cancel()

		data, err := o.Sub(ctx, addr)
		if err == nil {
			w.Header().Set("Content-Type", "text/plain")
			io.WriteString(w, data)
		} else {
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	})

	addr := "127.0.0.1:9451"
	log.WithField("local-addr", addr).Info("starting server")
	http.ListenAndServe(addr, nil)
}
