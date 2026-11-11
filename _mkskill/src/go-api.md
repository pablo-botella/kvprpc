---
mkskill:
  pos: 50
  in: readme
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
