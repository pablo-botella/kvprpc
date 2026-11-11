---
mkskill:
  pos: 60
  in: readme
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
