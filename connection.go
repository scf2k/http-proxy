package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

type ConnectionHandler interface {
	Handle() error
}

type ProxyConnectionHandler struct {
	Response http.ResponseWriter
	Request  *http.Request
	Server   *Server
}

func (c *ProxyConnectionHandler) Handle() error {
	if !c.auth() {
		c.Response.Header().Add("Proxy-Authenticate", "Basic")
		c.Response.WriteHeader(http.StatusProxyAuthRequired)
		return nil
	}
	if len(c.Server.Via) > 0 {
		c.Request.Header.Add("Via", c.Server.Via)
	}
	c.Request.Header.Del("Proxy-Authenticate")

	if c.Request.Method == http.MethodConnect {
		return c.tunnel()
	}
	if len(c.Request.URL.Host) == 0 {
		c.Request.URL.Host = c.Request.Host
		c.Request.URL.Scheme = "http"
	}

	dump, err := c.dumpRequest()
	if err == nil {
		defer dump.Close()
	}

	resp, err := http.DefaultTransport.RoundTrip(c.Request)
	if err != nil {
		http.Error(c.Response, err.Error(), http.StatusServiceUnavailable)
		return err
	}
	defer resp.Body.Close()
	c.copyHeaders(resp.Header)

	var reader io.Reader = resp.Body
	if dump != nil {
		resp.Header.WriteSubset(dump, nil)
		dump.WriteString("\r\n")
		reader = io.TeeReader(resp.Body, dump)
	}
	c.Response.WriteHeader(resp.StatusCode)
	io.Copy(c.Response, reader)

	return nil
}

func (c *ProxyConnectionHandler) dumpRequest() (dump *os.File, err error) {
	if !c.Server.Sniff {
		return
	}
	bodyBytes, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return
	}
	c.Request.Body.Close()
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	reqId := uuid.NewV1().String()
	log.Printf("dumping request to %s\n", reqId)

	dump, err = os.Create(reqId)
	if err != nil {
		return
	}
	dump.WriteString(fmt.Sprintf("%s %s\r\n", c.Request.Method, c.Request.URL))
	c.Request.Header.WriteSubset(dump, map[string]bool{"Via": true})
	dump.WriteString("\r\n")
	dump.Write(bodyBytes)
	dump.WriteString("\r\n\r\n")

	return
}

func (c *ProxyConnectionHandler) copyHeaders(src http.Header) {
	dest := c.Response.Header()
	for header, values := range src {
		for _, value := range values {
			dest.Add(header, value)
		}
	}
}

func (c *ProxyConnectionHandler) auth() bool {
	if len(c.Server.Auth) == 0 {
		return true
	}
	credHeader := c.Request.Header.Get("Proxy-Authorization")
	if len(credHeader) == 0 {
		return false
	}

	if !strings.HasPrefix(credHeader, "Basic ") {
		return false
	}

	credBase64 := credHeader[6:]

	if c.Server.Auth != credBase64 {
		return false
	}

	return true
}

func (c *ProxyConnectionHandler) tunnel() (err error) {
	var remote net.Conn
	if c.Server.Sniff {
		remote, err = tls.Dial("tcp", c.Request.Host, nil)
	} else {
		remote, err = net.DialTimeout("tcp", c.Request.Host, 10*time.Second)
	}
	if err != nil {
		http.Error(c.Response, err.Error(), http.StatusServiceUnavailable)
		return
	}

	c.Response.WriteHeader(http.StatusOK)
	hijacker, ok := c.Response.(http.Hijacker)
	if !ok {
		http.Error(c.Response, "hijacking is not supported", http.StatusInternalServerError)
		return errors.New("hijacking is not supported")
	}
	hijacked, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(c.Response, err.Error(), http.StatusServiceUnavailable)
		return
	}

	var client io.ReadWriteCloser = hijacked
	var dump *os.File
	if c.Server.Sniff {
		clientTls := tls.Server(hijacked, c.Server.config)

		err = clientTls.Handshake()
		if err != nil {
			http.Error(c.Response, err.Error(), http.StatusServiceUnavailable)
			return
		}
		client = clientTls

		reqId := uuid.NewV1().String()
		log.Printf("dumping request to %s\n", reqId)

		dump, err = os.Create(reqId)
		if err != nil {
			log.Printf("error opening dump file %s\n", err.Error())
		} else {
			defer dump.Close()
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go pipe(wg, remote, client, dump)
	go pipe(wg, client, remote, dump)

	wg.Wait()

	return nil
}

func pipe(wg *sync.WaitGroup, destination io.WriteCloser, source io.ReadCloser, sniffer io.Writer) {
	defer destination.Close()
	defer source.Close()
	defer wg.Done()

	var reader io.Reader = source
	if !reflect.ValueOf(sniffer).IsNil() {
		reader = io.TeeReader(source, sniffer)
	}
	io.Copy(destination, reader)
}
