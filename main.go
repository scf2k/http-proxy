package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
)

func main() {
	host := flag.String("host", ":", "address to listen on")
	port := flag.Int("port", 8080, "port to listen")

	flag.Parse()

	s := &Server{
		Host:               *host,
		Port:               *port,
		HandlerInitializer: createConnectionHandler,
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

func createConnectionHandler(conn net.Conn) ConnectionHandler {
	return &ProxyConnectionHandler{
		Connection: conn,
	}
}
