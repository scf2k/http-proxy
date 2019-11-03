package main

import (
	"encoding/base64"
	"flag"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	host := flag.String("host", ":", "address to listen on")
	port := flag.Int("port", 8080, "port to listen")
	via := flag.String("via", "", "value for the Via header")
	auth := flag.String("auth", "", "user:password to access the proxy")
	sniff := flag.Bool("sniff", false, "dump requests and responses to disk")

	flag.Parse()

	s := &Server{
		Host:  *host,
		Port:  *port,
		Via:   *via,
		Auth:  base64.StdEncoding.EncodeToString([]byte(*auth)),
		Sniff: *sniff,

		HandlerInitializer: func(w http.ResponseWriter, r *http.Request, s *Server) ConnectionHandler {
			return &ProxyConnectionHandler{
				Response: w,
				Request:  r,
				Server:   s,
			}
		},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			s.Stop()
			os.Exit(0)
		}
	}()

	s.Start()
}
