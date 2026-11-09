package main

import (
	"bufio"
	"bytes"
	"errors"
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
	Headers         map[string][]string
	Body            []byte
	QueryParams     url.Values
}

func isTokenChar(c byte) bool {
	if c >= 'a' && c <= 'z' ||
		c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' {
		return true
	}

	return strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c))
}

func validateHeader(key, value string) error {
	if key == "" {
		return fmt.Errorf("empty header name")
	}

	for i := 0; i < len(key); i++ {
		if !isTokenChar(key[i]) {
			return fmt.Errorf("invalid header name: %q", key)
		}
	}

	if strings.ContainsAny(key, "\r\n:") {
		return fmt.Errorf("invalid header name: %q", key)
	}

	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("invalid header value for %q", key)
	}

	return nil
}

func normalizeHeaders(headers map[string]string) (map[string]string, error) {
	normalized := make(map[string]string)

	for key, value := range headers {
		lowerKey := strings.ToLower(key)

		// validate key val
		if err := validateHeader(lowerKey, value); err != nil {
			return nil, err
		}

		// duplicate headers
		if _, exists := normalized[lowerKey]; exists {
			return nil, fmt.Errorf("duplicate response header: %s", key)
		}

		normalized[lowerKey] = value
	}

	return normalized, nil
}
func ensureHeaders(res *Response) {
	if res.Headers == nil {
		res.Headers = make(map[string]string)
	}
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
	ensureHeaders(res)

	// check if its HTTP Error, or produce a generic error

	var httpErr *HTTPError

	if errors.As(err, httpErr) {
		res.StatusCode = httpErr.StatusCode
		res.Body = []byte(httpErr.Message)
	} else {
		res.StatusCode = 500
		res.Body = []byte("Internal Server Error")
	}

	res.Headers["Content-Type"] = "text/plain"
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

	// ensure headers are initialized
	ensureHeaders(res)

	if req == nil || res == nil {
		return fmt.Errorf("nil request or response")
	}

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

	if cn == nil {
		return fmt.Errorf("nil connection")
	}
	if res == nil {
		return fmt.Errorf("nil response")
	}

	if res.StatusCode < 100 || res.StatusCode > 599 {
		return fmt.Errorf("invalid response status code: %d", res.StatusCode)
	}

	ensureHeaders(res)

	headers, err := normalizeHeaders(res.Headers)

	if err != nil {
		return err
	}

	res.Headers = headers

	res.Headers["content-length"] = strconv.Itoa(len(res.Body))
	res.Headers["connection"] = "close"

	var responseCn bytes.Buffer

	// Status line
	responseCn.WriteString(fmt.Sprintf(
		"HTTP/1.1 %d %s\r\n",
		res.StatusCode,
		StatusText(res.StatusCode),
	))

	for k, v := range res.Headers {
		responseCn.WriteString(k + ":" + " " + v + "\r\n")
	}
	responseCn.WriteString("\r\n")
	responseCn.Write(res.Body)

	_, writeErr := io.Copy(cn, &responseCn)
	if writeErr != nil {
		return fmt.Errorf("writing response: %w", err)
	}

	return nil
}

func NewServer() *Server {
	return &Server{}
}

func parseHeader(reader *bufio.Reader) (Request, error) {
	parsedReq := Request{}
	parsedReq.Headers = make(map[string][]string)

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
	parsedReq.Method = strings.ToUpper(splitReqLines[0])
	parsedReq.ResourcePath = splitReqLines[1]
	parsedReq.ProtocolVersion = splitReqLines[2]

	// validate for Method
	for i := 0; i < len(parsedReq.Method); i++ {
		if !isTokenChar(parsedReq.Method[i]) {
			return parsedReq, fmt.Errorf("invalid HTTP method")
		}
	}

	// validate for http version
	if parsedReq.ProtocolVersion != "HTTP/1.1" {
		return parsedReq, &HTTPError{
			StatusCode: 505,
			Message:    "HTTP Version Not Supported",
		}
	}

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
			fmt.Println("its a \r\n line ")
			break
		}
		headerParts := strings.SplitN(line, ":", 2) // split into exactly 2 ; ex : Content-Length: 11

		if len(headerParts) != 2 {
			return parsedReq, fmt.Errorf("Incorrect header: %q", line)
		}
		key := strings.ToLower(strings.TrimSpace(headerParts[0])) // lower for avoid case-insensitive
		value := strings.TrimSpace(headerParts[1])

		if key == "" {
			return parsedReq, fmt.Errorf("invalid header format")
		}

		// Validate header name
		for i := 0; i < len(key); i++ {
			if !isTokenChar(key[i]) {
				return parsedReq, fmt.Errorf("invalid header name: %q", key)
			}
		}

		parsedReq.Headers[key] = append(parsedReq.Headers[key], value)
	}
	return parsedReq, nil
}

func parseBody(reader *bufio.Reader, req *Request) ([]byte, error) {

	const maxBodySize = 1 << 20 // 1 MiB

	// Reject unsupported transfer encoding for now.
	if len(req.Headers["transfer-encoding"]) > 0 {
		return nil, &HTTPError{
			StatusCode: 501,
			Message:    "Transfer-Encoding is not supported",
		}
	}

	contentLengths := req.Headers["content-length"]

	if len(contentLengths) == 0 {
		req.Body = nil
		return nil, nil
	}

	if len(contentLengths) != 1 {
		return nil, &HTTPError{
			StatusCode: 400,
			Message:    "Multiple Content-Length headers",
		}
	}

	contentLength, err := strconv.Atoi(contentLengths[0])
	if err != nil || contentLength < 0 {
		return nil, &HTTPError{
			StatusCode: 400,
			Message:    "Invalid Content-Length",
		}
	}

	if contentLength > maxBodySize {
		return nil, &HTTPError{
			StatusCode: 413,
			Message:    "Request body too large",
		}
	}

	bodyBuffer := make([]byte, contentLength)

	// CHANGE 1: Capture the number of bytes read ('n') and use 'readErr' explicitly
	n, readErr := io.ReadFull(reader, bodyBuffer)
	if readErr != nil {
		// CHANGE 2: Your specific check for unexpected EOF
		if readErr == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("Interruption ; Read %d/%d bytes : %w", n, contentLength, readErr)
		}

		// Fallback for any other read errors
		return nil, &HTTPError{
			StatusCode: 400,
			Message:    "Incomplete request body",
			Err:        readErr,
		}
	}

	req.Body = bodyBuffer

	return bodyBuffer, nil
}

func handleConnection(cn net.Conn, wg *sync.WaitGroup, server *Server) {
	// cn management
	defer cn.Close()
	defer wg.Done()

	fmt.Println("Connection ", cn)
	res := &Response{
		Headers: make(map[string]string),
	}

	// request reading and parsing
	// Drain the HTTP request first. Closing a TCP socket while unread
	// request bytes remain in the receive buffer makes Windows send RST
	reader := bufio.NewReader(cn)

	finalReq, err := parseHeader(reader)
	if err != nil {
		log.Println("Error in parsing the header lines", err)
		server.HandleError(&HTTPError{
			StatusCode: 400,
			Message:    "Bad Request",
			Err:        err,
		}, res)
		_ = WriteResponse(cn, res)
		return
	}
	_, err = parseBody(reader, &finalReq)

	if err != nil {
		log.Println("Error in parsing the body", err)
		server.HandleError(err, res)
		_ = WriteResponse(cn, res)
		return
	}

	fmt.Println("We parsed and got this", finalReq)

	if err := server.HandleRequest(&finalReq, res); err != nil {
		server.HandleError(err, res)
	}

	if err := WriteResponse(cn, res); err != nil {
		log.Printf("Error writing response: %v", err)
	}
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
