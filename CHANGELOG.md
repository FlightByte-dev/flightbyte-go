## Unreleased

- Fix: a `POST`, `PATCH` or `DELETE` that gets a redirect is no longer followed. Go turns such a request into a `GET` without a body on a 301, 302 or 303, so `Subscriptions.Create` could return what the `GET` answered, with no error, although nothing was created. Only a `GET` or `HEAD` follows a redirect now, as in the Node and Dart clients; any other comes back as an `*APIError` with its 3xx status.
- Security: `VerifyWebhook` and `ConstructWebhookEvent` return the new `ErrEmptySecret` for an empty signing secret, and `WebhookHandler` panics on one. Anyone can sign with an empty key, so an unset environment variable used to accept forged events.
- `WebhookHandler` logs an error your function returns, and the new `WithErrorHandler` option sends it elsewhere. It used to answer 500 and drop the error.
- `ConstructWebhookEvent` returns an error for a verified body without an `id` or `type`, instead of an empty event.
- `APIError.Balance` and `APIError.ExpiresAt`, the prepaid credits left and when they expire, on a 402 with the new `"credits_exhausted"` code, sent when prepaid credits are used up. `Limit`, `Used` and `ResetsAt` are nil with that code.

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
