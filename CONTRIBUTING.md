# Contributing

Thanks for helping. Bug reports, fixes and docs improvements are all welcome. For a change to the public API, open an issue first so we can agree on the shape before you write it.

## Set up

Using the module needs Go 1.23 or later, and so does developing it.

```bash
git clone https://github.com/FlightByte-dev/flightbyte-go.git
cd flightbyte-go
go test ./...
```

The tests never touch the real API and need no key.

While the repository is private, the Go tools fetch it only for members of the FlightByte-dev organization, and only with:

```bash
go env -w GOPRIVATE=github.com/FlightByte-dev/*
```

## Before you open a pull request

```bash
gofmt -l .                         # prints nothing when formatted
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

CI runs the same checks, and a pull request needs all of them green.

- **Every exported identifier has a doc comment** that starts with its name, so it reads well on pkg.go.dev.
- **Every change has a test.** Requests go to an `httptest.Server` (see `helpers_test.go`), never to the network.
- **No dependencies.** The module uses only the standard library; keep it that way unless an issue agrees otherwise.
- **Add a line to `CHANGELOG.md`** under a `## Unreleased` heading for anything a user would notice.

## How the code maps to the API

The module mirrors the [OpenAPI document](https://flightbyte.dev/openapi.json): JSON tags are the API's own field names.

- **The API adds a field:** add it to the model and to the matching fixture in `testdata/`. Fixtures are decoded with `DisallowUnknownFields`, so a fixture field the model lacks fails the tests. A string the API may send as null stays a `string`; a number or time it may send as null is a pointer.
- **The API adds an enum value** (a phase, status, change type or event type): add a constant to `enums.go` and to its `Known` method. Until then the value arrives as is, which is why nothing breaks for users in the meantime.
- **The API adds an error code or status:** extend `newAPIError` in `errors.go`, add a sentinel if it needs one, and add a case to `errors_test.go`.
- **The API adds an endpoint:** add a method to the service, a test in `resources_test.go`, and a row to the README's API tables.

## Webhook signatures

`testdata/webhook_vectors.json` holds signatures made with Node's `createHmac`, exactly as the API signs deliveries, so the Go verifier is checked against the server and not only against itself. Do not edit those files by hand. To regenerate them, for example after adding a case to the script:

```bash
node tool/webhook_vectors.mjs
```

`FuzzParseSignature` fuzzes the signature header parser. Run it for a while after touching `webhook.go`:

```bash
go test -run '^$' -fuzz FuzzParseSignature -fuzztime 1m
```

## Commit messages

One line that says what changed and why it matters, in plain words, for example `Subscriptions can be listed by origin airport`. Add a body when the reason is not obvious from the diff.

## Releasing

Maintainers only.

1. Set `Version` in `version.go` to the new version.
2. Rename `## Unreleased` in `CHANGELOG.md` to the new version.
3. Push a tag `v<version>`, for example `v1.0.1`.

A Go module is released by its tag; there is nothing to upload. The release workflow checks that the tag matches `Version` and that the changelog has the section, runs the tests, and creates the GitHub release from the version's `CHANGELOG.md` section. To rehearse a release without creating one, run the workflow by hand from the Actions tab.

A tag is permanent once anyone has fetched the version through the Go module proxy: never move or reuse one. Fix a bad release with a new version.

## License

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE).
