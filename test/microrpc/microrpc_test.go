package microrpc_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pablo-botella/kvpack"
	"github.com/pablo-botella/microrpc"
	"github.com/pablo-botella/microrpc/test/api"
)

// server answers login (bob/secret, anything else is an error) and ping.
func server(t *testing.T) (*httptest.Server, *microrpc.Server, *[]error) {
	t.Helper()
	srv := microrpc.NewServer()
	var logged []error
	srv.ErrorLog = func(r *http.Request, command uint32, err error) { logged = append(logged, err) }
	microrpc.Handle(srv, api.CmdLogin, func(ctx context.Context, req *api.Login) (*api.LoginReply, error) {
		if req.User != "bob" || req.Password != "secret" {
			return nil, errors.New("wrong user or password")
		}
		if microrpc.Request(ctx) == nil || microrpc.Request(ctx).Header.Get("X-Token") != "t1" {
			return nil, errors.New("no token")
		}
		return &api.LoginReply{Name: "Bob", Level: 3}, nil
	})
	microrpc.Handle(srv, api.CmdPing, func(ctx context.Context, req *api.Ping) (*api.PingReply, error) {
		return &api.PingReply{At: time.Unix(1700000000, 0).UTC()}, nil
	})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, srv, &logged
}

func client(ts *httptest.Server) *microrpc.Client {
	c := microrpc.NewClient(ts.URL)
	c.Header = http.Header{"X-Token": {"t1"}}
	return c
}

func TestCall(t *testing.T) {
	ts, _, _ := server(t)
	c := client(ts)
	var reply api.LoginReply
	if err := c.Call(context.Background(), &api.Login{User: "bob", Password: "secret"}, &reply); err != nil {
		t.Fatal(err)
	}
	if reply.Name != "Bob" || reply.Level != 3 {
		t.Fatalf("%+v", reply)
	}
	var pong api.PingReply
	if err := c.Call(context.Background(), &api.Ping{}, &pong); err != nil || pong.At.Unix() != 1700000000 {
		t.Fatalf("%v %+v", err, pong)
	}
}

func TestErrorReply(t *testing.T) {
	ts, _, logged := server(t)
	c := client(ts)
	var reply api.LoginReply
	err := c.Call(context.Background(), &api.Login{User: "bob", Password: "nope"}, &reply)
	var re *microrpc.ReplyError
	if !errors.As(err, &re) || re.Text != "wrong user or password" || re.Command != api.CmdLogin {
		t.Fatalf("%T %v", err, err)
	}
	if len(*logged) != 1 {
		t.Errorf("ErrorLog saw %d errors", len(*logged))
	}
	// the same through CallMessage: the reply is a message carrying the error
	m, err := c.CallMessage(context.Background(), (&api.Login{User: "x"}).Pack())
	if err != nil {
		t.Fatal(err)
	}
	if text, bad := m.ErrorText(); !bad || text != "wrong user or password" {
		t.Errorf("%q %v", text, bad)
	}
}

func TestMissingFieldAndUnknownCommand(t *testing.T) {
	ts, _, _ := server(t)
	c := client(ts)
	m, err := c.CallMessage(context.Background(), kvpack.New(api.CmdLogin).AddText(1, "bob"))
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := m.ErrorText(); text != "login: missing password" {
		t.Errorf("missing field: %q", text)
	}
	m, err = c.CallMessage(context.Background(), kvpack.New(99))
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := m.ErrorText(); text != "unknown command 99" || m.Command != 99 {
		t.Errorf("unknown command: %q %d", text, m.Command)
	}
}

func TestTransportErrors(t *testing.T) {
	ts, _, _ := server(t)
	// not a message: 400
	resp, err := http.Post(ts.URL, microrpc.ContentType, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("garbage: %s", resp.Status)
	}
	// GET: 405
	resp, err = http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET: %s", resp.Status)
	}
	// a client sees a status other than 200 as a *StatusError
	c := microrpc.NewClient(ts.URL + "/nowhere")
	c.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 502, Status: "502 Bad Gateway", Body: http.NoBody, Request: r}, nil
	})}
	var se *microrpc.StatusError
	if err := c.Call(context.Background(), &api.Ping{}, &api.PingReply{}); !errors.As(err, &se) || se.StatusCode != 502 {
		t.Errorf("%T %v", err, err)
	}
}

func TestMaxSize(t *testing.T) {
	ts, srv, _ := server(t)
	srv.MaxSize = 64
	big := kvpack.New(api.CmdLogin).AddText(1, strings.Repeat("x", 100)).AddText(2, "secret")
	resp, err := http.Post(ts.URL, microrpc.ContentType, bytes.NewReader(big.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("big request: %s", resp.Status)
	}
	// and a client refuses a reply over its own bound
	srv.MaxSize = 0
	c := client(ts)
	c.MaxSize = 21
	if err := c.Call(context.Background(), &api.Ping{}, &api.PingReply{}); !errors.Is(err, kvpack.ErrTooLarge) {
		t.Errorf("big reply: %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
