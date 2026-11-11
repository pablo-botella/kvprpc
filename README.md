# microrpc

Minimal RPC over HTTP for [kvpack](https://github.com/pablo-botella/kvpack)
messages: a **Go** server that dispatches each request to the handler of its
command, and a client that posts a request and reads the reply. A call is
one `POST` whose body is the request message and whose response body is the
reply message.

```sh
go get github.com/pablo-botella/microrpc
```

- **One POST per call.** Body `application/x-kvpack`, request in, reply out.
  No paths per command, no query strings: the command number is in the
  message.
- **Typed on both sides.** Declare the calls once in a `.kvp` file; the
  `kvpack` command generates the structs, and `Handle` and `Call` take them
  as they are.
- **Errors are replies.** What a handler returns as an error reaches the
  client as a kvpack error reply on HTTP status `200`; any other status is a
  transport failure.
- **Plain HTTP.** The server is an `http.Handler`: mount it on a route, put
  it behind any middleware. The client takes an `*http.Client` for
  timeouts, proxies and TLS.
- **Simple to reach from anywhere.** Any HTTP client that can post bytes and
  any kvpack implementation (Go, C, Harbour) can call it.

---

## Table of contents

- [Quick start](#quick-start)
- [Go API](#go-api)
- [Protocol](#protocol)
- [Design notes](#design-notes)
- [License](#license)

---

## Quick start

Declare the calls in a `.kvp` file:

```
command login = 1
    user      text  = 1
    password  text  = 2
reply login
    name      text  = 1
    level     int32 = 2 optional
```

and generate the Go code with the `kvpack` command:

```sh
kvpack gen -go api/api_kvp.go -pkg api api.kvp
```

### Server

```go
srv := microrpc.NewServer()
microrpc.Handle(srv, api.CmdLogin, func(ctx context.Context, req *api.Login) (*api.LoginReply, error) {
    u, err := authenticate(req.User, req.Password)
    if err != nil {
        return nil, errors.New("wrong user or password") // the client sees this text
    }
    return &api.LoginReply{Name: u.Name, Level: u.Level}, nil
})
http.Handle("/rpc", srv)
```

A request missing a required field gets an error reply without reaching
the handler; an unknown command gets one too.

### Client

```go
c := microrpc.NewClient("https://www.example.com/rpc")
var reply api.LoginReply
err := c.Call(ctx, &api.Login{User: "bob", Password: "secret"}, &reply)
var re *microrpc.ReplyError
switch {
case errors.As(err, &re):
    // the server said no: re.Text
case err != nil:
    // transport, HTTP status (*microrpc.StatusError) or a bad reply
}
```

---

## Go API

### Server

```go
func NewServer() *Server
func Handle[Req, Rep any, PReq interface{ *Req; Unpacker }, PRep interface{ *Rep; Packer }](
    s *Server, command uint32, fn func(ctx context.Context, req PReq) (PRep, error))
func (s *Server) HandleMessage(command uint32, fn HandlerFunc)
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)
func Request(ctx context.Context) *http.Request
```

`Handle` registers a typed handler: the request is unpacked into a new
`Req` and the `Rep` returned is packed. `HandleMessage` works on raw
`*kvpack.Message` values. A nil reply with a nil error answers an empty
reply of the same command. Register every handler before serving.

`Request(ctx)` gives a handler the HTTP request it is answering, for its
headers (a token, say) or its remote address.

| field | |
|---|---|
| `MaxSize int` | largest request accepted, in bytes; `0` is `DefaultMaxSize` (1 MB) |
| `ErrorLog func(r *http.Request, command uint32, err error)` | sees every error a handler returns, before it goes to the client |

### Client

```go
func NewClient(url string) *Client
func (c *Client) Call(ctx context.Context, req Packer, reply Unpacker) error
func (c *Client) CallMessage(ctx context.Context, req *kvpack.Message) (*kvpack.Message, error)
```

`Call` returns an error reply as a `*ReplyError` (`Command`, `Text`).
`CallMessage` returns the reply message as it came, error replies
included: check `ErrorText` on it.

| field | |
|---|---|
| `URL string` | where the server is mounted |
| `HTTP *http.Client` | does the requests; nil is `http.DefaultClient` |
| `Header http.Header` | added to every request |
| `MaxSize int` | largest reply accepted, in bytes; `0` is `DefaultMaxSize` |

### Errors

| error | meaning |
|---|---|
| `*ReplyError` | the server answered with an error reply |
| `*StatusError` | an HTTP status other than 200: the call did not reach a handler |
| wraps a kvpack error | the reply was not a valid message (`kvpack.ErrTooLarge` over `MaxSize`) |
| anything else | the transport (`net/http`, a cancelled context...) |

---

## Protocol

A call is an HTTP `POST` to the server's URL:

- request body: one kvpack message, `Content-Type: application/x-kvpack`;
- response body: one kvpack message, the reply, with the same command.

| status | when | body |
|---|---|---|
| `200` | every request that is a valid message | the reply: the handler's, or an error reply (key 0) for an unknown command, a missing required field or a handler error |
| `400` | the body is not a valid kvpack message | text |
| `405` | the method is not `POST` | text |
| `413` | the body is over the server's `MaxSize` | text |

Anything else (`404`, `502`...) comes from whatever sits in between. A
client in any language needs only this: post the bytes, check for `200`,
read one message, check its key 0.

---

## Design notes

**Why HTTP.** It crosses proxies and firewalls, it brings TLS, and every
platform has a client for it. A call is a single request and its response,
so nothing about HTTP gets in the way.

**Why one URL.** The command number already says what is asked. One route
keeps the server one `http.Handler` and the client one URL.

**Why errors on status 200.** The message is the protocol and the status
reports only the transport. A client has two cases, not a table of
statuses: it either got a message, and reads key 0, or it did not.

**Why the handler's error text goes to the client.** It is the simple
rule, and what a handler returns is what it means to say. A handler that
could leak an internal error (a database message, a path) returns its own
text instead and leaves the details to `ErrorLog`.

**Why no generator of its own.** The structs the `kvpack` command writes
already have `Pack` and `Unpack`; the generic `Handle` and `Call` take them
as they are. A `.kvp` change is one regeneration, no second tool.

**What is deliberately out.** Streaming, asynchronous calls, retries,
authentication (a header and `Request(ctx)` carry a token; middleware
does the rest) and service discovery.

---

## License

MIT.
