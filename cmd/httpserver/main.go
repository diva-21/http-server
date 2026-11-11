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
	"sort"
	"strconv"
	"strings"
	"sync"
)

type HTTPError struct {
	StatusCode int
	Message    string
	Err        error
}

type Handler func(*Request, *Response) error

type Server struct {
	routes map[string]map[string]Handler // path->method->handler
	mu     sync.RWMutex
}
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

const (
	maxRequestLine = 8 * 1024
	maxHeaderSize  = 32 * 1024
	maxHeaderCount = 100
	maxBodySize    = 1 << 20 // 1 MiB

	maxTrailerSize  = 32 * 1024
	maxTrailerCount = 100
)

// sample registered routes

func homeHandler(req *Request, res *Response) error {

	ensureHeaders(res)

	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("Welcome to my HTTP server")

	return nil
}

func getUsers(req *Request, res *Response) error {

	ensureHeaders(res)

	fmt.Println("Requested QueryParams: ", req.QueryParams)
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("Getting all users")

	return nil
}

func createUser(req *Request, res *Response) error {

	ensureHeaders(res)

	res.StatusCode = 201
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("User created successfully")

	return nil
}

func deleteUser(req *Request, res *Response) error {

	ensureHeaders(res)

	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("User deleted successfully")

	return nil
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

	if errors.As(err, &httpErr) {
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

	if s == nil {
		return fmt.Errorf("nil server")
	}

	if req == nil || res == nil {
		return fmt.Errorf("nil request or response")
	}

	// ensure headers are initialized
	ensureHeaders(res)

	if req.Method == "" {
		return &HTTPError{
			StatusCode: 400,
			Message:    "Bad Request",
		}
	}

	//read lock
	s.mu.RLock()

	// first look for the path
	methodRoutes, pathExists := s.routes[req.ResourcePath]
	if !pathExists {

		s.mu.RUnlock() // unlock, no routes present for this path

		return &HTTPError{
			StatusCode: 404,
			Message:    "Not Found",
		}
	}

	//next look for the requested method for the path
	handler, methodsExists := methodRoutes[req.Method]

	if !methodsExists {

		allowedMethods := make([]string, 0, len(methodRoutes))

		for method := range methodRoutes {
			allowedMethods = append(allowedMethods, method)
		}

		s.mu.RUnlock()

		sort.Strings(allowedMethods)

		res.Headers["Allow"] = strings.Join(allowedMethods, ", ")

		return &HTTPError{
			StatusCode: 405,
			Message:    "Method Not Allowed",
		}

	}

	s.mu.RUnlock() // dont wait the handler to complete its execution; we need to just the right handler.

	// execute the matched handler
	return handler(req, res)

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
		return fmt.Errorf("writing response: %w", writeErr)
	}

	return nil
}

func NewServer() *Server {
	return &Server{
		routes: make(map[string]map[string]Handler),
	}
}

func (s *Server) Handle(method, path string, handler Handler) error {
	if s == nil {
		return fmt.Errorf("nil server")
	}
	if method == "" {
		return fmt.Errorf("empty http method")
	}

	if path == "" || !strings.HasPrefix(path, "/") {
		return fmt.Errorf("invalid route path: %q", path)
	}

	if strings.ContainsAny(path, "?#") {
		// query param would be handled in parsing logic and not here.
		// frags not allowed for now
		return fmt.Errorf("route path must not contain query or fragment")
	}

	if handler == nil {
		return fmt.Errorf("nil handler")
	}

	// same validation logic
	for i := 0; i < len(method); i++ {
		if !isTokenChar(method[i]) {
			return fmt.Errorf("invalid HTTP method: %q", method)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// init
	if s.routes == nil {
		s.routes = make(map[string]map[string]Handler)
	}

	// init method map
	if _, exists := s.routes[path]; !exists {
		s.routes[path] = make(map[string]Handler)
	}

	// check if its already present
	if _, exists := s.routes[path][method]; !exists {
		fmt.Errorf("route already registered : %s %s ", method, path)
	}

	// register the handler
	s.routes[path][method] = handler

	return nil
}

func ReadLineWithLimit(reader *bufio.Reader, maxLimit int) (string, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')

		if len(part)+len(line) > maxLimit {
			return "", fmt.Errorf("line exceeded maximum size")
		}
		line = append(line, part...)

		if err == nil {
			// got one full line , break
			break
		}
		if err != bufio.ErrBufferFull {
			// other error in reading
			return "", err
		}
		// err == ErrBufferFull means, continue & go to next iteration and stich until we get "\n"
	}

	// when we exit the loop, we got one full line, now check conditions on that full line
	// 1. ends with '\n'
	// 2. last one before char is '\r'
	// 3. at least size should be 2 ; (\r\n) is min req in a line

	if len(line) < 2 || line[len(line)-1] != '\n' || line[len(line)-2] != '\r' {
		return "", fmt.Errorf("invalid line ending")
	}

	return string(line[:len(line)-2]), nil // strip CRLF (\r\n)

}
func parseHeader(reader *bufio.Reader) (Request, error) {
	parsedReq := Request{}
	parsedReq.Headers = make(map[string][]string)

	// 1. Parsing the first request line (method one)
	// requestLine, err := reader.ReadString('\n') // old
	requestLine, err := ReadLineWithLimit(reader, maxRequestLine)

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
	parsedReq.ProtocolVersion = splitReqLines[2]

	parsedReq.ResourcePath = splitReqLines[1]

	target, err := url.ParseRequestURI(splitReqLines[1])
	if err != nil ||
		target.IsAbs() ||
		target.Host != "" ||
		target.Fragment != "" ||
		!strings.HasPrefix(target.Path, "/") {

		return parsedReq, &HTTPError{
			StatusCode: 400,
			Message:    "Invalid request target",
		}
	}

	// store only the resource path
	parsedReq.ResourcePath = target.Path // contains /users

	// extract query parameters
	parsedReq.QueryParams, err = url.ParseQuery(target.RawQuery) // rawQuery contains id=1&name=Sai
	if err != nil {
		return parsedReq, &HTTPError{
			StatusCode: 400,
			Message:    "Invalid query parameters",
			Err:        err,
		}
	}

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

	headerCount := 0
	totalHeaderSize := 0
	for {
		// line, err := reader.ReadString('\n') // old
		line, err := ReadLineWithLimit(reader, maxHeaderSize)
		if err != nil {
			log.Println("err reading request", err)
			return parsedReq, fmt.Errorf("Error in reading the header line %w", err)
		}

		totalHeaderSize += len(line) + 2 // include CRLF

		if totalHeaderSize > maxHeaderSize {
			return parsedReq, &HTTPError{
				StatusCode: 431,
				Message:    "Request Header Fields Too Large",
			}
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			fmt.Println("its a \r\n line ")
			break
		}

		headerCount++

		if headerCount > maxHeaderCount {
			return parsedReq, &HTTPError{
				StatusCode: 431,
				Message:    "too many request headers",
			}
		}
		headerParts := strings.SplitN(line, ":", 2) // split into exactly 2 ; ex : Content-Length: 11

		if len(headerParts) != 2 {
			return parsedReq, fmt.Errorf("incorrect header: %q", line)
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

// just consume from the reader, dont store for now.
func consumeTrailers(reader *bufio.Reader) error {

	totalSize := 0
	count := 0

	for {
		line, err := ReadLineWithLimit(reader, maxTrailerSize)
		if err != nil {
			return &HTTPError{
				StatusCode: 400,
				Message:    "invalid chunk trailers",
				Err:        err,
			}
		}

		totalSize += len(line) + 2 // include CRLF

		if totalSize > maxTrailerSize {
			return &HTTPError{
				StatusCode: 431,
				Message:    "trailer section too large",
			}
		}

		// Empty line means trailer section is finished.
		if line == "" {
			return nil
		}

		count++

		if count > maxTrailerCount {
			return &HTTPError{
				StatusCode: 431,
				Message:    "too many trailer fields",
			}
		}

		key, value, found := strings.Cut(line, ":")

		if !found || key == "" {
			return &HTTPError{
				StatusCode: 400,
				Message:    "invalid trailer field",
			}
		}

		for i := 0; i < len(key); i++ {
			if !isTokenChar(key[i]) {
				return &HTTPError{
					StatusCode: 400,
					Message:    "invalid trailer field name",
				}
			}
		}

		value = strings.Trim(value, " \t")

		for i := 0; i < len(value); i++ {
			if value[i] < 32 && value[i] != '\t' ||
				value[i] == 127 {
				return &HTTPError{
					StatusCode: 400,
					Message:    "invalid trailer field value",
				}
			}
		}

		// Do not accept framing or routing fields as trailers.
		switch strings.ToLower(key) {
		case "content-length", "transfer-encoding", "host", "trailer":
			return &HTTPError{
				StatusCode: 400,
				Message:    "forbidden trailer field",
			}
		}
	}
}
func parseChunkedBody(reader *bufio.Reader, maxBodySize int) ([]byte, error) {
	// extract chunk size
	// convert chunk size (hexa) to int
	// check chunk size == 0; if yes, then check if they're any trailers present
	// check max size limit
	// read chunk size no. of bytes to body
	// CRITICAL: at last , post reading the above bytes, check it ends with CRLF

	var body []byte

	for {

		line, err := ReadLineWithLimit(reader, maxRequestLine)
		if err != nil {
			return nil, &HTTPError{
				StatusCode: 400,
				Message:    "invalid chunk line size",
				Err:        err,
			}
		}

		// extract chunk size from "10;ext1;ext2=hello"
		// chunkSize;extraData;....
		sizeText, _, _ := strings.Cut(line, ";")
		if sizeText == "" {
			return nil, &HTTPError{StatusCode: 400, Message: "empty chunk size"}
		}

		// chunk size ( hexa to 64 int )
		chunkSize, err := strconv.ParseUint(sizeText, 16, 64)
		if err != nil {
			return nil, &HTTPError{StatusCode: 400, Message: "invalid chunk size", Err: err}
		}

		// if chunkSize == 0 , then check for trailers; Trailers = late-calculated metadata
		if chunkSize == 0 {
			// check for trailers
			if err := consumeTrailers(reader); err != nil {
				return nil, err
			}
			return body, nil
		}

		// check max size limit
		if chunkSize > uint64(maxBodySize-len(body)) {
			return nil, &HTTPError{StatusCode: 413, Message: "request body too large"}
		}

		//read chunkSize no. of bytes to body
		chunk := make([]byte, int(chunkSize))

		if _, err := io.ReadFull(reader, chunk); err != nil {
			return nil, &HTTPError{StatusCode: 400, Message: "incomplete chunk data", Err: err}
		}

		body = append(body, chunk...)

		// chunk data must be followed by CRLF
		var ending [2]byte
		if _, err = io.ReadFull(reader, ending[:]); err != nil || ending != [2]byte{'\r', '\n'} {
			return nil, &HTTPError{StatusCode: 400, Message: "invalid chunk ending", Err: err}
		}

	}
}
func parseBody(reader *bufio.Reader, req *Request) ([]byte, error) {

	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	transferEncodings := req.Headers["transfer-encoding"]
	contentLengths := req.Headers["content-length"]

	if len(transferEncodings) > 0 && len(contentLengths) > 0 {
		// both cant exist at once
		return nil, &HTTPError{
			StatusCode: 400,
			Message:    "Transfer-Encoding & Content-Length both cant be present",
		}
	}

	// handle transfer encoding
	if len(transferEncodings) > 0 {
		// rm dups
		if len(transferEncodings) != 1 {
			return nil, &HTTPError{
				StatusCode: 400,
				Message:    "multiple Transfer-Encoding headers",
			}
		}

		encoded := strings.TrimSpace(transferEncodings[0])
		// Transfer-Encoding: chunked

		if !strings.EqualFold(encoded, "chunked") {
			// only chunked is accepted
			return nil, &HTTPError{
				StatusCode: 501,
				Message:    "Transfer-Encoding is not supported",
			}
		}

		// retrieving body from the stream 5\r\nHello\r\n0
		body, err := parseChunkedBody(reader, maxBodySize)
		if err != nil {
			return nil, err
		}

		req.Body = body
		return body, nil

	}

	// handle content length
	if len(contentLengths) == 0 {
		req.Body = nil
		return nil, nil
	}

	if len(contentLengths) != 1 {
		return nil, &HTTPError{
			StatusCode: 400,
			Message:    "multiple Content-Length headers",
		}
	}

	contentLength, err := strconv.Atoi(contentLengths[0])
	if err != nil || contentLength < 0 {
		return nil, &HTTPError{
			StatusCode: 400,
			Message:    "invalid Content-Length",
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

	if err := server.Handle("GET", "/", homeHandler); err != nil {
		log.Fatal(err)
	}

	if err := server.Handle("GET", "/users", getUsers); err != nil {
		log.Fatal(err)
	}

	if err := server.Handle("POST", "/users", createUser); err != nil {
		log.Fatal(err)
	}

	if err := server.Handle("DELETE", "/users", deleteUser); err != nil {
		log.Fatal(err)
	}

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
