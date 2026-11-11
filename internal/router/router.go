package router

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/diva-21/http-server/internal/httperror"
	"github.com/diva-21/http-server/internal/parser"
	"github.com/diva-21/http-server/internal/response"
)

type Handler func(*parser.Request, *response.Response) error

type Router struct {
	routes map[string]map[string]Handler // path->method->handler
	mu     sync.RWMutex
}

func isTokenChar(c byte) bool {
	if c >= 'a' && c <= 'z' ||
		c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' {
		return true
	}

	return strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c))
}

func New() *Router {
	return &Router{
		routes: make(map[string]map[string]Handler),
	}
}

func (r *Router) Handle(method, path string, handler Handler) error {
	if r == nil {
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
	r.mu.Lock()
	defer r.mu.Unlock()

	// init
	if r.routes == nil {
		r.routes = make(map[string]map[string]Handler)
	}

	// init method map
	if _, exists := r.routes[path]; !exists {
		r.routes[path] = make(map[string]Handler)
	}

	// check if its already present
	if _, exists := r.routes[path][method]; !exists {
		fmt.Errorf("route already registered : %s %s ", method, path)
	}

	// register the handler
	r.routes[path][method] = handler

	return nil
}

func (r *Router) HandleRequest(req *parser.Request, res *response.Response) error {

	if r == nil {
		return fmt.Errorf("nil server")
	}

	if req == nil || res == nil {
		return fmt.Errorf("nil request or response")
	}

	// ensure headers are initialized
	response.EnsureHeaders(res)

	if req.Method == "" {
		return &httperror.HTTPError{
			StatusCode: 400,
			Message:    "Bad Request",
		}
	}

	//read lock
	r.mu.RLock()

	// first look for the path
	methodRoutes, pathExists := r.routes[req.ResourcePath]
	if !pathExists {

		r.mu.RUnlock() // unlock, no routes present for this path

		return &httperror.HTTPError{
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

		r.mu.RUnlock()

		sort.Strings(allowedMethods)

		res.Headers["Allow"] = strings.Join(allowedMethods, ", ")

		return &httperror.HTTPError{
			StatusCode: 405,
			Message:    "Method Not Allowed",
		}

	}

	r.mu.RUnlock() // dont wait the handler to complete its execution; we need to just the right handler.

	// execute the matched handler
	return handler(req, res)

}
