package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/diva-21/http-server/internal/httperror"
	"github.com/diva-21/http-server/internal/parser"
	"github.com/diva-21/http-server/internal/response"
)

func handleConnection(cn net.Conn, wg *sync.WaitGroup, server *Server) {
	// cn management
	defer cn.Close()
	defer wg.Done()

	fmt.Println("Connection ", cn)
	res := &response.Response{
		Headers: make(map[string]string),
	}

	// request reading and parsing
	// Drain the HTTP request first. Closing a TCP socket while unread
	// request bytes remain in the receive buffer makes Windows send RST
	reader := bufio.NewReader(cn)

	finalReq, err := parser.ParseHeader(reader)
	if err != nil {
		log.Println("Error in parsing the header lines", err)
		server.HandleError(&httperror.HTTPError{
			StatusCode: 400,
			Message:    "Bad Request",
			Err:        err,
		}, res)
		_ = response.WriteResponse(cn, res)
		return
	}
	_, err = parser.ParseBody(reader, &finalReq)

	if err != nil {
		log.Println("Error in parsing the body", err)
		server.HandleError(err, res)
		_ = response.WriteResponse(cn, res)
		return
	}

	fmt.Println("We parsed and got this", finalReq)

	if err := server.HandleRequest(&finalReq, res); err != nil {
		server.HandleError(err, res)
	}

	if err := response.WriteResponse(cn, res); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}
