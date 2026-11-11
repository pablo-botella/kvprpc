---
mkskill:
  pos: 90
  in: readme
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
