---
mkskill:
  pos: 10
  in: readme
---

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
