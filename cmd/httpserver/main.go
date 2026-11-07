package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

type HTTPError struct {
	StatusCode int
	Message    string
	Err        error
}

type Server struct{} // for now empty. later will add Routes
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

type Request struct {
	Method          string
	ResourcePath    string
	ProtocolVersion string
	FieldNames     map[string][]string
	Body            []byte
	QueryParams     url.Values
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}

	return e.Message
}

func (s *Server) HandleError(
	err error,
	res *Response,
) {
	if res.StatusCode > 100 && res.StatusCode < 600 {
		res.Body = []byte(StatusText(res.StatusCode))
	} else {
		res.Body = []byte("Unknown error from the server")
	}
}
func StatusText(statusCode int) string {
	switch statusCode {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 408:
		return "Request Timeout"
	case 413:
		return "Content Too Large"
	case 414:
		return "URI Too Long"
	case 431:
		return "Request Header Fields Too Large"
	case 500:
		return "Internal Server Error"
	case 501:
		return "Not Implemented"
	case 505:
		return "HTTP Version Not Supported"
	default:
		return "Unknown Status"
	}
}

func (s *Server) HandleGET(req *Request, res *Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("GET request received")
	return nil
}

func (s *Server) HandlePOST(req *Request, res *Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("POST request received")
	return nil
}

func (s *Server) HandlePUT(req *Request, res *Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("PUT request received")
	return nil
}

func (s *Server) HandleDELETE(req *Request, res *Response) error {
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("DELETE request received")
	return nil
}
func (s *Server) HandleRequest(req *Request, res *Response) error {

	method := strings.TrimSpace(strings.ToUpper(req.Method))
	if method == "" {
		log.Println("Method cant be empty", method)
		return fmt.Errorf("Method empty error :%q", method)
	}

	switch method {

	case "GET":
		return s.HandleGET(req, res)

	case "POST":
		return s.HandlePOST(req, res)

	case "PUT":
		return s.HandlePUT(req, res)

	case "DELETE":
		return s.HandleDELETE(req, res)

	default:
		return &HTTPError{
			StatusCode: 405,
			Message:    "Method Not Allowed",
		}
	}
}

func WriteResponse(cn net.Conn, res *Response) error {
	// response formatting
	res.Headers["Content-Length"] = strconv.Itoa(len(res.Body))

	var responseStr strings.Builder

	responseStr.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", res.StatusCode, StatusText(res.StatusCode)))

	for k, v := range res.Headers {
		responseStr.WriteString(k + ":" + " " + v + "\r\n")
	}
	responseStr.WriteString("\r\n")
	responseStr.Write(res.Body)

	_, err := cn.Write([]byte(responseStr.String()))
	if err != nil {
		log.Println("err in writing the response", err)
		return fmt.Errorf("Error in writing response %v\n", err)
	}
	fmt.Println("Response written successfully")
	return nil
}

func NewServer() *Server {
	return &Server{}
}

func parseHeader(reader *bufio.Reader) (Request, error) {
	parsedReq := Request{}
	parsedReq.FieldNames = make(map[string][]string)

	// 1. Parsing the first request line (method one)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return parsedReq, fmt.Errorf("Error in reading the request line %w", err)
	}

	requestLine = strings.TrimRight(requestLine, "\r\n")
	splitReqLines := strings.Fields(requestLine)

	if len(splitReqLines) != 3 {
		return parsedReq, fmt.Errorf("Invalid request %q", requestLine)
	}

	// METHOD /RESOURCE-PATH PROTOCOL-VERSION 	\r\n
	parsedReq.Method = splitReqLines[0]
	parsedReq.ResourcePath = splitReqLines[1]
	parsedReq.ProtocolVersion = splitReqLines[2]

	// 2. Parsing the field names / headers ( key : values ) until the blank line
	// example Host: x.com , Content-Length: 11
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println("err reading request", err)
			return parsedReq, fmt.Errorf("Error in reading the header line %w", err)
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// this only works for GET, not for POST ( it would have request body after end of line )
			fmt.Println("its a \r\n line ")
			break
		}
		headerParts := strings.SplitN(line, ":", 2) // split into exactly 2 ; ex : Content-Length: 11

		if len(headerParts) != 2 {
			return parsedReq, fmt.Errorf("Incorrect header: %q", line)
		}
		key := strings.ToLower(strings.TrimSpace(headerParts[0])) // lower for avoid case-insensitive
		value := strings.TrimSpace(headerParts[1])

		if key == "" || value == "" {
			return parsedReq, fmt.Errorf("Invalid key-value in header. Key: %q , value : %q", key, value)
		}

		parsedReq.FieldNames[key] = append(parsedReq.FieldNames[key], value)
	}
	return parsedReq, nil
}

func parseBody(reader *bufio.Reader, req *Request) ([]byte, error) {
	// we are checking whether content-length is present or not. if not then no body

	contentLengthStrRes := req.FieldNames["content-length"]
	if len(contentLengthStrRes) == 0 {
		// no content length header is present
		fmt.Println("No content length header is present")
		return nil, nil
	}
	contentLengthStr := contentLengthStrRes[0] // content-length will be once in header
	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil || contentLength < 0 {
		return nil, fmt.Errorf("Invalid content-length")
	}

	// make []byte of contentLength
	bodyBuffer := make([]byte, contentLength)

	n, readErr := io.ReadFull(reader, bodyBuffer)

	if readErr != nil {

		if readErr == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("Interruption ; Read %d/%d bytes : %w", n, contentLength, readErr)
		}
		return nil, readErr
	}
	req.Body = bodyBuffer
	return bodyBuffer, nil

}
func handleConnection(cn net.Conn, wg *sync.WaitGroup, server *Server) {
	// cn management
	defer cn.Close()
	defer wg.Done()

	fmt.Println("Connection ", cn)

	// request reading and parsing
	// Drain the HTTP request first. Closing a TCP socket while unread
	// request bytes remain in the receive buffer makes Windows send RST
	reader := bufio.NewReader(cn)

	finalReq, parseErr := parseHeader(reader)
	if parseErr != nil {
		log.Println("Error in parsing the header lines", parseErr)
		return
	}
	bodyBuf, parseBodyErr := parseBody(reader, &finalReq)

	if parseBodyErr != nil {
		log.Println("Error in parsing the body", parseBodyErr)
		return
	}

	fmt.Println("We parsed and got this", finalReq)
	fmt.Printf("And body is %q\n", string(bodyBuf))

	resInit := &Response{
		Headers: make(map[string]string),
	}

	if reqHandleErr := server.HandleRequest(&finalReq, resInit); reqHandleErr != nil {
		server.HandleError(reqHandleErr, resInit)
	}

	WriteResponse(cn, resInit)
}

func startServer() {
	// my tcp needs to the following:
	//
	// Connection management
	// Reading the HTTP request headers & parsing
	// Body reading & parsing
	// Response formatting
	server := NewServer()
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
		go handleConnection(cn, &wg, server)
	}
}
func main() {
	startServer()
}
