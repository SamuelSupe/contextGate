# Contributing

Small, focused changes are welcome. Start an issue for a new adapter or a change to the authorization model so its boundary can be agreed first.

## Development

Use Go 1.26, a C/C++ toolchain, Node.js 24 and Docker. SQLite and DuckDB require CGO. Tests require `MCPDBHUB_TEST_DATABASE_URL` pointing to a disposable PostgreSQL database whose role can create/drop schemas. Tests isolate temporary configurations by schema and remove only their own schemas. Build the embedded UI before running Go tests:

```sh
make ui
export MCPDBHUB_TEST_DATABASE_URL='postgres://test:REPLACE_ME@127.0.0.1:5432/mcpdbhub_test?sslmode=disable'
go test ./...
go test -race ./...
npm --prefix web test
node --test web/test/query.test.mjs
make build
```

The database matrix provisions isolated containers. Follow [validation](docs/validation.md) to create its `mcpdbhub-dev` container and network, then run `python3 scripts/matrix.py <product>`. Do not point fixture scripts at production data. Database compatibility claims need an independent passing product/version result; protocol compatibility alone is insufficient.

Keep queries read-only, apply authorization to metadata as well as data, preserve native types and parameter precision, and cancel work when access is revoked. Add a regression test when it protects meaningful behavior or a security boundary. Test rendered UI changes in local Chrome; do not add a browser automation framework for a small UI fix.

Before opening a pull request, remove credentials and local fixtures, explain the observable change, and report what you actually tested. English is the default for the README, documentation, UI strings, issue and pull request templates, and release notes. Keep translations in files ending in `.zh-CN.md`, with explicit language links. Update both README and installation-guide languages when user-facing instructions change.

## License

Contributions intentionally submitted for inclusion in ContextGate are licensed under the [Apache License, Version 2.0](LICENSE). Preserve applicable third-party copyright, license and attribution notices.
