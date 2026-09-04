# Offer HTTP/2 at the TLS terminator

## Problem

Lighthouse "Modern HTTP" reports the protocol the *browser* negotiated with
the TLS terminator, not the hop from the Gateway to the pod. Cilium Gateway
HTTPS listeners do not advertise `h2` unless `gatewayAPI.enableAlpn` is true.
With it false, Chrome stays on HTTP/1.1 even though TLS already terminates
on the Gateway.

HTTP/2 from Envoy to a cleartext Go `ListenAndServe` pod needs h2c. Go does
not speak h2c unless the process wraps the server. Setting
`appProtocol: kubernetes.io/h2c` on that Service causes protocol errors.

HTTP/3 needs a QUIC listener. Cilium Gateway does not expose one. Opening
UDP 443 on the UDM does not enable h3.

## Practice

- Enable `gatewayAPI.enableAlpn: true` on the Cilium install that owns the
  public Gateway. Restart `cilium-operator`, then `cilium-envoy` if openssl
  still prints `No ALPN negotiated`.
- Leave website Services without `appProtocol: kubernetes.io/h2c`.
- Confirm with:

  ```sh
  openssl s_client -connect example.com:443 -servername example.com -alpn h2 </dev/null 2>/dev/null | grep ALPN
  curl -sI --http2 https://example.com/
  ```

  Expect `ALPN protocol: h2` and `HTTP/2 200`.

This is cluster config, not an application CSS change. Apps still benefit:
one stylesheet plus HTTP/2 multiplexing is what made the remaining
render-blocking cost small.

## Anti-pattern

Do not try to "fix HTTP/1.1" by combining files only and leaving ALPN off.
Do not send h2c to a standard library Go HTTP server.
Do not punch UDP 443 expecting Cilium Gateway to speak HTTP/3.

## How we learned it

Live `cilium-config` had `enable-gateway-api-alpn: "false"`. After setting it
true and restarting Envoy, `genosservers.com` negotiated HTTP/2. Lighthouse
Modern HTTP went from a 200ms HTTP/1.1 finding to a pass.
