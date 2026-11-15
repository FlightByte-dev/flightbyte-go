## 1.0.0

- First release.
- `Client` with `Flights` (`Get`, `Timeline`), `Subscriptions` (`Create`, `List`, `All`, `Get`, `Update`, `Delete`, `Pause`, `Resume`, `SendTest`, `Deliveries`, `Events`, `Secret`, `RotateSecret`) and `Countries` (`List`, `Get`, `FlagURL`).
- Typed models for flights, subscriptions, deliveries, timelines and webhook events. Enum values added to the API later arrive as is, and `Known()` reports whether this version has a constant for them.
- `*APIError` for every API error status, matching `ErrBadRequest`, `ErrNotFound`, `ErrRateLimited` and the other sentinels with `errors.Is`, including the `"credits_exhausted"` reason of a 402 with its `Balance` and `ExpiresAt`, and `*ConnectionError` for requests that got no response.
- `VerifyWebhook`, `ConstructWebhookEvent`, `SignWebhook` and `WebhookHandler` for `x-flightbyte-signature` webhook signatures. An empty signing secret is refused with `ErrEmptySecret`, and `WebhookHandler` panics on one, since anyone can sign with an empty key. `WebhookHandler` logs an error your function returns, or hands it to `WithErrorHandler`.
- `Client.LastRateLimit` from the `X-RateLimit-*` headers, returned as a copy.
- A redirect is followed only for a `GET` or `HEAD`, and only on the same host and scheme, so the API key stays with the API and a write is never replayed as a `GET`.
- No dependencies outside the standard library.
