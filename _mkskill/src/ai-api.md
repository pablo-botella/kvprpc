---
mkskill:
  pos: 210
  in: ai*
---

## Package API

### Server

```go
srv := microrpc.NewServer()
srv.MaxSize = 1 << 20 // request bound; 0 = DefaultMaxSize (1 MB), over it = 413
srv.ErrorLog = func(r *http.Request, cmd uint32, err error) { log.Print(cmd, err) }
microrpc.Handle(srv, api.CmdLogin, func(ctx context.Context, req *api.Login) (*api.LoginReply, error) {
    tok := microrpc.Request(ctx).Header.Get("X-Token") // the HTTP request
    return &api.LoginReply{Name: "Bob"}, nil           // or nil, err: an error reply with err.Error()
})
srv.HandleMessage(cmd, func(ctx context.Context, m *kvpack.Message) (*kvpack.Message, error) { ... })
http.Handle("/rpc", srv) // *Server is an http.Handler
```

`Handle` unpacks into a new `Req` (a missing or mistyped required field is
an error reply, `fn` is not called) and packs the `Rep`. A nil reply with a
nil error answers an empty reply of the same command.

### Client

```go
c := microrpc.NewClient(url)
c.HTTP = &http.Client{Timeout: 10 * time.Second} // nil = http.DefaultClient
c.Header = http.Header{"X-Token": {tok}}         // added to every request
c.MaxSize = 0                                    // reply bound; 0 = DefaultMaxSize
err := c.Call(ctx, &api.Login{User: "bob"}, &reply) // typed
m, err := c.CallMessage(ctx, kvpack.New(cmd))        // raw: error replies come back as messages
```

Errors from `Call`: `*ReplyError{Command, Text}` (an error reply),
`*StatusError{StatusCode, Status}` (HTTP status not 200), a wrapped kvpack
error (bad reply, `kvpack.ErrTooLarge`), the `Unpack` error of the reply
(missing required field), or the transport's.

### Wire

| status | when |
|---|---|
| 200 | valid request message; body = reply (maybe an error reply, key 0) |
| 400 | body not a valid message |
| 405 | method not POST |
| 413 | body over `MaxSize` |
