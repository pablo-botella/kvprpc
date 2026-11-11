---
mkskill:
  pos: 40
  in: readme
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
