package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

type Server struct {
	Host               string
	Port               int
	HandlerInitializer func(net.Conn) ConnectionHandler

	listener      net.Listener
	shutdownMutex sync.Mutex
	shuttingDown  bool
	workers       sync.WaitGroup
}

func (s *Server) Start() (err error) {
	if !strings.HasSuffix(s.Host, ":") {
		s.Host += ":"
	}
	s.listener, err = net.Listen("tcp", fmt.Sprintf("%s%d", s.Host, s.Port))
	if err != nil {
		return
	}

	defer s.listener.Close()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.shuttingDown {
				break
			}
			log.Printf("error accepting connection %v", err)
			continue
		}
		if s.shuttingDown && conn != nil {
			conn.Close()
		} else {
			log.Printf("accepted connection from %v", conn.RemoteAddr())

			go s.handle(conn)
		}
	}
	return
}

func (s *Server) Stop() {
	s.shutdownMutex.Lock()
	if s.shuttingDown {
		s.shutdownMutex.Unlock()
		return
	}
	s.shuttingDown = true
	s.shutdownMutex.Unlock()

	log.Printf("waiting for connections to close\n")
	s.workers.Wait()
	s.listener.Close()
}

func (s *Server) handle(conn net.Conn) error {
	s.workers.Add(1)

	defer func() {
		log.Printf("closing connection from %v", conn.RemoteAddr())
		conn.Close()
		s.workers.Done()
	}()

	if s.HandlerInitializer != nil {
		handler := s.HandlerInitializer(conn)
		if handler != nil {
			return handler.Handle()
		}
	}

	return nil
}
