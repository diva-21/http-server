package server

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/diva-21/http-server/internal/parser"
	"github.com/diva-21/http-server/internal/response"
	"github.com/diva-21/http-server/internal/router"
)

type Server struct {
	router *router.Router
}

func NewServer() *Server {
	return &Server{
		router: router.New(),
	}
}

func (s *Server) Handle(method, path string, handler router.Handler) error {
	if s == nil {
		return fmt.Errorf("nil server")
	}
	return s.router.Handle(method, path, handler)
}

func (s *Server) HandleGET(req *parser.Request, res *response.Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("GET request received")
	return nil
}

func (s *Server) HandlePOST(req *parser.Request, res *response.Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("POST request received")
	return nil
}

func (s *Server) HandlePUT(req *parser.Request, res *response.Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("PUT request received")
	return nil
}

func (s *Server) HandleDELETE(req *parser.Request, res *response.Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("DELETE request received")
	return nil
}

func (s *Server) HandleRequest(req *parser.Request, res *response.Response) error {
	return s.router.HandleRequest(req, res)
}

func (s *Server) Start() {
	// my tcp needs to the following:
	//
	// Connection management
	// Reading the HTTP request headers & parsing
	// Body reading & parsing
	// Response formatting
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("error ", err)
	}
	defer listener.Close()

	wg := sync.WaitGroup{}

	fmt.Println("Running on 8080 port")

	for {
		cn, err := listener.Accept()
		if err != nil {
			log.Println("Error in accept", err)
			continue
		}
		wg.Add(1)
		go handleConnection(cn, &wg, s)
	}
}
