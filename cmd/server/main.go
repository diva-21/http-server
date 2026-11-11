package main

import (
	"fmt"
	"log"

	"github.com/diva-21/http-server/internal/parser"
	"github.com/diva-21/http-server/internal/response"
	"github.com/diva-21/http-server/internal/server"
)

// sample registered routes

func homeHandler(req *parser.Request, res *response.Response) error {

	response.EnsureHeaders(res)

	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("Welcome to my HTTP server")

	return nil
}

func getUsers(req *parser.Request, res *response.Response) error {

	response.EnsureHeaders(res)

	fmt.Println("Requested QueryParams: ", req.QueryParams)
	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("Getting all users")

	return nil
}

func createUser(req *parser.Request, res *response.Response) error {

	response.EnsureHeaders(res)

	res.StatusCode = 201
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("User created successfully")

	return nil
}

func deleteUser(req *parser.Request, res *response.Response) error {

	response.EnsureHeaders(res)

	res.StatusCode = 200
	res.Headers["Content-Type"] = "text/plain"
	res.Body = []byte("User deleted successfully")

	return nil
}

func main() {
	srv := server.NewServer()

	if err := srv.Handle("GET", "/", homeHandler); err != nil {
		log.Fatal(err)
	}

	if err := srv.Handle("GET", "/users", getUsers); err != nil {
		log.Fatal(err)
	}

	if err := srv.Handle("POST", "/users", createUser); err != nil {
		log.Fatal(err)
	}

	if err := srv.Handle("DELETE", "/users", deleteUser); err != nil {
		log.Fatal(err)
	}

	srv.Start()
}
