// Package microrpc carries kvpack messages over HTTP: a server that
// dispatches each request to the handler of its command, and a client that
// posts a request and reads the reply. A call is one POST whose body is the
// request message and whose response body is the reply message, under the
// media type application/x-kvpack.
//
// With the structs the kvpack command generates from a .kvp declaration
// (they have Pack and Unpack), both sides are typed:
//
//	srv := microrpc.NewServer()
//	microrpc.Handle(srv, api.CmdLogin, func(ctx context.Context, req *api.Login) (*api.LoginReply, error) {
//		...
//	})
//	http.Handle("/rpc", srv)
//
//	c := microrpc.NewClient("https://host/rpc")
//	var reply api.LoginReply
//	err := c.Call(ctx, &api.Login{User: "bob"}, &reply)
//
// An error a handler returns travels as a kvpack error reply (key 0) with
// HTTP status 200: the message is the protocol, the status reports only the
// transport. The client returns it as a *ReplyError.
package microrpc

// The documentation is not automatic — this makes it: `go generate ./...`
// rebuilds every artifact from _mkskill/ (README, AGENTS.md and the skill;
// the tool version is pinned by go.mod), and the golden test fails when
// anything went stale. The second line propagates the version-spec's
// destinations (__publish.bat) without touching the version.
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill -q build
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill -q -vbuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/pablo-botella/kvpack"
)

// ContentType is the media type of a request and of its reply.
const ContentType = "application/x-kvpack"

// DefaultMaxSize bounds a request a Server reads, and a reply a Client
// reads, when their MaxSize is 0.
const DefaultMaxSize = 1 << 20

// Packer is a request or reply that builds its message: what the kvpack
// generator writes for every block.
type Packer interface {
	Pack() *kvpack.Message
}

// Unpacker is a request or reply that fills itself from a message.
type Unpacker interface {
	Unpack(m *kvpack.Message) error
}

// HandlerFunc answers a request message with a reply message. A nil reply
// with a nil error answers an empty reply of the request's command.
type HandlerFunc func(ctx context.Context, req *kvpack.Message) (*kvpack.Message, error)

// Server dispatches each request to the handler registered for its
// command. It is an http.Handler: mount it on a route. Register handlers
// before serving; the map is not guarded.
type Server struct {
	// MaxSize bounds a request in bytes (0: DefaultMaxSize). A larger one
	// gets HTTP 413.
	MaxSize int
	// ErrorLog, when set, sees every error a handler returns, before it
	// goes to the client as an error reply.
	ErrorLog func(r *http.Request, command uint32, err error)

	handlers map[uint32]HandlerFunc
}

// NewServer returns a server with no handlers.
func NewServer() *Server {
	return &Server{handlers: map[uint32]HandlerFunc{}}
}

// HandleMessage registers fn for command, on raw messages.
func (s *Server) HandleMessage(command uint32, fn HandlerFunc) {
	if s.handlers == nil {
		s.handlers = map[uint32]HandlerFunc{}
	}
	s.handlers[command] = fn
}

// Handle registers a typed handler for command: the request is unpacked
// into a new Req (a missing required field is answered as an error reply
// without calling fn) and the reply fn returns is packed.
func Handle[Req, Rep any, PReq interface {
	*Req
	Unpacker
}, PRep interface {
	*Rep
	Packer
}](s *Server, command uint32, fn func(ctx context.Context, req PReq) (PRep, error)) {
	s.HandleMessage(command, func(ctx context.Context, m *kvpack.Message) (*kvpack.Message, error) {
		req := PReq(new(Req))
		if err := req.Unpack(m); err != nil {
			return nil, err
		}
		rep, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}
		if rep == nil {
			return nil, nil
		}
		return rep.Pack(), nil
	})
}

type requestKey struct{}

// Request returns the HTTP request a handler is answering, for its headers
// or remote address; nil outside a Server.
func Request(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestKey{}).(*http.Request)
	return r
}

// ServeHTTP answers a POST whose body is one message. A body that is not a
// valid message gets 400, a method other than POST 405, a body over
// MaxSize 413. Everything else is a 200 carrying a reply: the handler's,
// or an error reply (unknown command, a handler's error).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "microrpc: POST only", http.StatusMethodNotAllowed)
		return
	}
	req, err := kvpack.Read(r.Body, maxSize(s.MaxSize))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, kvpack.ErrTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, "microrpc: "+err.Error(), status)
		return
	}
	reply := s.dispatch(r, req)
	b := reply.Encode()
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	_, _ = w.Write(b)
}

func (s *Server) dispatch(r *http.Request, req *kvpack.Message) *kvpack.Message {
	fn := s.handlers[req.Command]
	if fn == nil {
		return kvpack.NewError(req.Command, fmt.Sprintf("unknown command %d", req.Command))
	}
	ctx := context.WithValue(r.Context(), requestKey{}, r)
	reply, err := fn(ctx, req)
	if err != nil {
		if s.ErrorLog != nil {
			s.ErrorLog(r, req.Command, err)
		}
		return kvpack.NewError(req.Command, err.Error())
	}
	if reply == nil {
		return kvpack.New(req.Command)
	}
	return reply
}

// Client posts requests to one URL.
type Client struct {
	// URL is where the server is mounted.
	URL string
	// HTTP does the requests; nil means http.DefaultClient. Timeouts,
	// proxies and TLS settings live there.
	HTTP *http.Client
	// Header is added to every request (an authorization token, say).
	Header http.Header
	// MaxSize bounds a reply in bytes (0: DefaultMaxSize).
	MaxSize int
}

// NewClient returns a client for url.
func NewClient(url string) *Client {
	return &Client{URL: url}
}

// ReplyError is an error reply: the server answered, and its answer is
// this text.
type ReplyError struct {
	Command uint32
	Text    string
}

func (e *ReplyError) Error() string { return e.Text }

// StatusError is an HTTP status other than 200: the call did not reach a
// handler (bad request, not found, a proxy in the way...).
type StatusError struct {
	StatusCode int
	Status     string
}

func (e *StatusError) Error() string { return "microrpc: " + e.Status }

// CallMessage posts req and returns the reply message, error replies
// included (check reply.ErrorText). Transport failures, a status other
// than 200 (*StatusError) and a reply that is not a valid message are the
// errors.
func (c *Client) CallMessage(ctx context.Context, req *kvpack.Message) (*kvpack.Message, error) {
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(req.Encode()))
	if err != nil {
		return nil, err
	}
	for k, v := range c.Header {
		hr.Header[k] = v
	}
	hr.Header.Set("Content-Type", ContentType)
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(hr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, &StatusError{StatusCode: resp.StatusCode, Status: resp.Status}
	}
	reply, err := kvpack.Read(resp.Body, maxSize(c.MaxSize))
	if err != nil {
		return nil, fmt.Errorf("microrpc: reply: %w", err)
	}
	return reply, nil
}

// Call posts req and unpacks the reply into reply. An error reply comes
// back as a *ReplyError; a reply missing a required field as the error of
// its Unpack.
func (c *Client) Call(ctx context.Context, req Packer, reply Unpacker) error {
	m, err := c.CallMessage(ctx, req.Pack())
	if err != nil {
		return err
	}
	if text, bad := m.ErrorText(); bad {
		return &ReplyError{Command: m.Command, Text: text}
	}
	return reply.Unpack(m)
}

func maxSize(n int) int {
	if n <= 0 {
		return DefaultMaxSize
	}
	return n
}
