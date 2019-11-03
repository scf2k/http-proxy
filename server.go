package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

type Server struct {
	Host               string
	Port               int
	HandlerInitializer func(http.ResponseWriter, *http.Request, *Server) ConnectionHandler
	Via                string
	Auth               string
	Sniff              bool

	listener      *http.Server
	shutdownMutex sync.Mutex
	shuttingDown  bool
}

func (s *Server) Start() (err error) {
	if !strings.HasSuffix(s.Host, ":") {
		s.Host += ":"
	}

	s.listener = &http.Server{
		Addr:    fmt.Sprintf("%s%d", s.Host, s.Port),
		Handler: http.HandlerFunc(s.handle),
	}
	log.Fatal(s.listener.ListenAndServe())

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
	s.listener.Close()
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	log.Printf("serving connection from %v", r.RemoteAddr)

	defer func() {
		log.Printf("closing connection with %v", r.RemoteAddr)
	}()

	if s.HandlerInitializer != nil {
		handler := s.HandlerInitializer(w, r, s)
		if handler != nil {
			err := handler.Handle()
			if err != nil {
				log.Printf("[%v]: %s\n", r.RemoteAddr, err.Error())
			}
		}
	}
}
