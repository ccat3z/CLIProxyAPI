# Removed: TLS / HTTPS Listen Support

The `custom` branch deployment only ever serves plain HTTP (TLS is terminated by an outer reverse proxy / load balancer). The built-in HTTPS listen mode of the API server has been removed to shrink the server's configuration surface and listener code path.

## What was removed

### Configuration schema

The top-level `tls:` block and its backing Go type are gone. The plain-HTTP `host` / `port` settings are unchanged.

| Removed | Where | Notes |
| --- | --- | --- |
| `tls.enable` / `tls.cert` / `tls.key` | `config.example.yaml` | The whole `tls:` block was deleted. |
| `TLSConfig` struct | `internal/config/config.go` | Held `Enable`, `Cert`, `Key`. |
| `Config.TLS` field | `internal/config/config.go` | Top-level config field. |
| `TLSConfig` / `TLS` type aliases | `sdk/config/config.go` | Public SDK re-exports of the removed type. |

### Listener code

| Removed | Where | Notes |
| --- | --- | --- |
| TLS branch of `Server.Start()` | `internal/api/server.go` | The `ListenAndServeTLS`-equivalent path that loaded the cert/key pair, built a `tls.Config`, configured HTTP/2, and wrapped the listener with `tls.NewListener`. Only the plain-HTTP path remains. |
| `crypto/tls` and `golang.org/x/net/http2` imports | `internal/api/server.go` | Were only used by the TLS branch. |
| TLS handshake / ALPN routing in `routeMuxConnection` | `internal/api/protocol_multiplexer.go` | The `*tls.Conn` type assertion, `Handshake()`, and `NegotiatedProtocol` ALPN dispatch. Plain HTTP and Redis RESP protocol detection on the shared listener are retained. |
| `bufferedConn.ConnectionState()` helper | `internal/api/buffered_conn.go` | Existed only to surface `tls.ConnectionState` for TLS connections. |

### Safemode warning server

The example-API-key warning server (`internal/safemode`) also had an HTTPS mode mirroring the main server's TLS branch. It is now plain-HTTP only:

- `WarningServerURL` no longer produces `https://` URLs.
- `StartExampleAPIKeyWarningServer` no longer loads a cert/key pair or wraps its listener with `tls.NewListener`.
- `sdk/cliproxy`'s home overlay no longer copies a `TLS` field onto the merged config.

The corresponding `TestWarningServerURL` case that asserted an `https://` URL was dropped; the remaining case still covers IPv6 host bracketing over plain HTTP.

## What was kept

Client-side TLS is unrelated to the server listen mode and is intentionally retained:

- `internal/config/home.go` `HomeTLSConfig` and `internal/home/` — TLS used by the outbound home/control-plane Redis connection.
- The `-home-jwt` flag (`cmd/server/main.go`) — mTLS certificate bootstrap for the home connection.

## Rationale

The deployment terminates TLS at the edge; serving HTTPS from the Go process itself was unused. Removing it drops a config block, a listener branch, the `crypto/tls` and `golang.org/x/net/http2` imports from the API server, and the ALPN-handshake path from the protocol multiplexer, leaving a single plain-HTTP listen path.
