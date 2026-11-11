---
mkskill:
  pos: 240
  in: ai*
---

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
