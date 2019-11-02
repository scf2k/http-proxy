package main

import (
	"bufio"
	"log"
	"net"
	"strings"
)

type ConnectionHandler interface {
	Handle() error
}

type ProxyConnectionHandler struct {
	Connection net.Conn
}

func (c *ProxyConnectionHandler) Handle() error {
	reader := bufio.NewReader(c.Connection)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("%v(%v)", err, c.Connection.RemoteAddr())
			return err
		}
		log.Printf("recieved from client %s:'%s'", c.Connection.RemoteAddr(), strings.TrimRight(line, "\r\n"))
	}
	return nil
}
