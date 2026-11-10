## 0.1.2

- Security: the client no longer follows a redirect to another host or from https to http. Go copies the `X-API-Key` header on every redirect, so a 3xx answer could hand the API key to another server, or send it in the clear. Such a redirect now comes back as an `*APIError` with its 3xx status. A client passed with `WithHTTPClient` is copied, not changed, and its own `CheckRedirect` still applies within the same host.
- `Client.LastRateLimit` returns a copy of each window too, so changing the result no longer changes the client's own state or races with requests in flight.

## 0.1.1

- `FlightTimeline.Count`, the number of entries. The API sends it with every timeline, and the model now keeps it.

## 0.1.0

- First release.
- `Client` with `Flights` (`Get`, `Timeline`), `Subscriptions` (`Create`, `List`, `All`, `Get`, `Update`, `Delete`, `Pause`, `Resume`, `SendTest`, `Deliveries`, `Events`, `Secret`, `RotateSecret`) and `Countries` (`List`, `Get`, `FlagURL`).
- Typed models for flights, subscriptions, deliveries and webhook events. Enum values added to the API later arrive as is, and `Known()` reports whether this version has a constant for them.
- `*APIError` for every API error status, matching `ErrBadRequest`, `ErrNotFound`, `ErrRateLimited` and the other sentinels with `errors.Is`, and `*ConnectionError` for requests that got no response.
- `VerifyWebhook`, `ConstructWebhookEvent`, `SignWebhook` and `WebhookHandler` for `x-flightbyte-signature` webhook signatures.
- `Client.LastRateLimit` from the `X-RateLimit-*` headers.
- No dependencies outside the standard library.
