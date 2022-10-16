package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

type ConnectionInfoHandler struct{}

func (h *ConnectionInfoHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		log.Printf("%v", err)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(map[string]interface{}{
		"method":  req.Method,
		"host":    req.Host,
		"path":    req.URL.String(),
		"headers": req.Header,
		"content": string(body),
	})
}

func main() {
	var (
		port uint
	)

	flag.UintVar(&port, "port", 80, "server port")
	flag.Parse()

	address := fmt.Sprintf(":%d", port)
	netListener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("listen %v", address)
	httpServer := &http.Server{Handler: &ConnectionInfoHandler{}}
	httpServer.Serve(netListener)
}
