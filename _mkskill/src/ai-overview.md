---
mkskill:
  pos: 200
  in: ai*
---

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
