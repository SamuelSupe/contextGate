# Release process

Release artifacts come from the same versioned Docker build as the runtime image. `scripts/dist.py` requires a clean committed checkout, injects its commit into the CLI, copies the binary and required C++ libraries, and packages the bilingual documentation, notices and examples. Build on Linux or through OrbStack/Docker; CGO requires the target architecture's C/C++ toolchain.

The default `README.md`, installation guide, release notes and templates use English. Chinese documentation lives in `.zh-CN.md` files. Existing `.en.md` URLs point readers to the canonical English pages. Published tags and distribution assets retain their original contents; documentation updates are delivered on the default branch and in subsequent packages.

## Prepare and verify

1. Update `internal/version/version.go`, `web/package.json` / lockfile, the Docker image version label, changelog and download links.
2. Build the UI with `make ui`. Run `go test ./...`, `go test -race ./...`, and `node --test web/test/query.test.mjs` in the appropriate environments.
3. For adapter or query-boundary changes, provision and test each supported database with `scripts/matrix.py`, then run `scripts/publish-verification.py`. Every newly published matrix report must match the current implementation digest. For changes outside the database execution path, run focused integration checks and clearly identify historical matrix evidence; do not relabel it as a new full-matrix run. Keep only a dedicated PostgreSQL fixture for package checks: `MCPDBHUB_KEEP_FIXTURES=1 python3 scripts/matrix.py postgres`.
4. Refresh dependency notices when dependencies change. The Go collection uses `github.com/google/go-licenses/v2@v2.0.1`; preserve the reviewed MIT-0 / vendored Thrift supplements and native DuckDB notices, and inspect new unknown-license results. Frontend notices come from installed, locked packages. Collected source material stays under `third_party/licenses/_go` so it is not treated as application code by Go.
5. Capture UI changes through local Chrome using disposable data, update both README languages, and commit the release source.

## Build dist

```sh
python3 scripts/dist.py --version 0.3.0
# A single architecture, optionally using a trusted build-download CA:
python3 scripts/dist.py --version 0.3.0 --arch arm64 --build-ca /path/to/trusted-ca.pem
```

The script writes `dist/mcpdbhub-0.3.0-linux-{arm64,amd64}.tar.gz` and `dist/SHA256SUMS`. These archives require glibc 2.36 or newer. Keep the launcher, executable and private libraries together. `BUILD.json` includes the source commit, platform, binary hash and source image ID. On an arm64 machine, amd64 builds run through Docker's architecture support; do not describe emulation as native hardware validation.

## Validate the actual archives

With the retained, isolated PostgreSQL fixture available:

```sh
python3 scripts/check-package.py \
  --archive dist/mcpdbhub-0.3.0-linux-arm64.tar.gz \
  --otlp --report artifacts/release-0.3.0/dist-arm64.json
python3 scripts/check-package.py \
  --archive dist/mcpdbhub-0.3.0-linux-amd64.tar.gz \
  --otlp --report artifacts/release-0.3.0/dist-amd64.json
```

Each command unpacks into a fresh directory, verifies its manifest/hash, and runs its launcher inside a clean Debian container. It checks version, UID, runtime libraries, no Node runtime, bootstrap, HTTP/stdio MCP discovery and PostgreSQL queries, restart persistence, revocation and password recovery. The semantic flow requires a real trial before publication, executes templates over both HTTP and stdio, denies native queries in templates-only mode and recovers publication proof after restart. The ontology flow imports semantic v1, publishes a source mapping in v2 without changing the execution version, checks filtered Agent discovery, preserves ontology context across HTTP/stdio and restart, and verifies bilingual ontology guides. With `--otlp`, it also starts an isolated Collector 0.160.0 and verifies HTTP/protobuf, gRPC, encrypted-header redaction, pending records across restart, and recovery after receiver downtime, plus template and ontology correlation attributes and parameter redaction. It cleans up its own containers, volume and extraction directory. `--image mcpdbhub:0.3.0-arm64` verifies an image instead of an archive.

## Publish

Push the reviewed commit to `main` and require both CI architecture jobs to pass. Create an annotated `v0.3.0` tag at that commit. Create the GitHub Release as a draft, attach both archives, `SHA256SUMS` and the actual validation records, then download those assets again and compare hashes. Verify the local, remote branch and dereferenced tag commits before publishing the Release as latest.

Do not attach configuration databases, master keys, fixture manifests, tokens, local logs or build caches. Do not move a published tag to another commit. Keep historical verification records tied to the source they actually tested.
