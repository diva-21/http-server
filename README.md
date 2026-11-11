# HTTP Server in Go

A simple HTTP/1.1 server built from scratch using TCP in Go, without relying on Go's `net/http` package.

I built this project to understand how HTTP works under the hood — from accepting TCP connections and parsing HTTP requests to handling routes and writing responses back to clients.

The implementation follows the basics of [RFC 9112](https://datatracker.ietf.org/doc/html/rfc9112).

## Features

- **TCP server:** Accepts connections and handles each client in a separate goroutine.
- **HTTP/1.1 parsing:** Parses request lines, headers, query parameters, and request bodies.
- **Request body handling:** Supports both `Content-Length` and chunked transfer encoding.
- **Routing:** Supports exact path and HTTP method matching, with `404 Not Found` and `405 Method Not Allowed` handling.
- **Concurrent route management:** Uses `sync.RWMutex` to safely register and access routes while requests are being handled.
- **HTTP responses:** Writes status lines, headers, and response bodies with proper `Content-Length` and connection handling.

## Example Routes

| Method | Path | Description |
|---|---|---|
| GET | `/` | Home route |
| GET | `/users` | Get users |
| POST | `/users` | Create a user |
| DELETE | `/users` | Delete a user |

## Getting Started

Clone the repository and run the server:

```bash
go run ./cmd/server
```

The server starts on port `8080`.

## Testing with curl

Try the following requests to see the server in action:

**1. Home route**

```bash
curl http://localhost:8080/
```

**2. Request with query parameters**

```bash
curl "http://localhost:8080/users?id=1&name=Sai"
```

**3. Create a user**

```bash
curl -X POST http://localhost:8080/users
```

**4. Delete a user**

```bash
curl -X DELETE http://localhost:8080/users
```

**5. Try an unsupported HTTP method (405)**

```bash
curl -i -X PUT http://localhost:8080/users
```

**6. Request a route that doesn't exist (404)**

```bash
curl -i http://localhost:8080/missing
```

## What I Learned

Working on this project helped me understand how HTTP/1.1 works at a lower level, especially how HTTP messages are exchanged over TCP.

It also gave me hands-on experience with Go's networking and concurrency features, request parsing, routing, and response handling.

## What's Next?

Some features I'd like to explore next:

- Dynamic route parameters (e.g. `/users/:id`)
- Persistent (keep-alive) connections to handle multiple requests over a single TCP connection
- Gzip response compression when supported by the client
- Support for multiple content encodings and choosing the best one based on the client's `Accept-Encoding` header
- Graceful connection closure and handling of the `Connection` header