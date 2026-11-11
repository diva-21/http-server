package server

import (
	"errors"

	"github.com/diva-21/http-server/internal/httperror"
	"github.com/diva-21/http-server/internal/response"
)

func (s *Server) HandleError(
	err error,
	res *response.Response,
) {
	response.EnsureHeaders(res)

	// check if its HTTP Error, or produce a generic error

	var httpErr *httperror.HTTPError

	if errors.As(err, &httpErr) {
		res.StatusCode = httpErr.StatusCode
		res.Body = []byte(httpErr.Message)
	} else {
		res.StatusCode = 500
		res.Body = []byte("Internal Server Error")
	}

	res.Headers["Content-Type"] = "text/plain"
}
