# microrpc

## Overview

`microrpc` is a minimal RPC over HTTP for
[kvpack](https://github.com/pablo-botella/kvpack) messages. A call is one
`POST` with body `application/x-kvpack`: the request message in, the reply
message out. The server dispatches on the message's command number.

Module: `github.com/pablo-botella/microrpc` — standard library plus
`github.com/pablo-botella/kvpack`.

```sh
go get github.com/pablo-botella/microrpc
```

### When to use

- A Go server exposing commands to Go, C or Harbour clients, where gRPC is
  too heavy and JSON too loose.
- Calls declared in a `.kvp` file: the structs `kvpack gen -go` writes plug
  straight into `Handle` and `Call`.

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

## Rules & gotchas

- **The handler's error text reaches the client verbatim.** Do not return
  internal errors (database messages, paths) from a handler: return a text
  meant for the client and log the details, or let `ErrorLog` see them.
- **Errors travel on status 200.** Check for `*ReplyError` (or `ErrorText`
  after `CallMessage`); a `*StatusError` means the call never reached a
  handler.
- **Register handlers before serving.** The handler map is not guarded;
  registering while serving is a data race.
- **One command, one handler.** Registering a command again replaces the
  previous handler.
- **Unknown commands are error replies**, not 404: the text is
  `unknown command N`.
- **Bounds on both sides.** `Server.MaxSize` and `Client.MaxSize` default to
  1 MB; raise them deliberately for large replies.
- **No transport options of its own.** Timeouts, proxies and TLS belong to
  the `*http.Client` in `Client.HTTP`; authentication to headers and
  middleware.
- **Codes come from the `.kvp`.** Use the generated `Cmd*` constants and
  structs (`kvpack gen -go`); do not number commands by hand on one side
  only.
