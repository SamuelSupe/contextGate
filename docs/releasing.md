# Release process

Release artifacts come from the same versioned Docker build as the runtime image. `scripts/dist.py` requires a clean committed checkout, injects its commit into the CLI, copies the binary and required C++ libraries, and packages the bilingual documentation, notices and examples. Build on Linux or through OrbStack/Docker; CGO requires the target architecture's C/C++ toolchain.

The default `README.md`, installation guide, release notes and templates use English. Chinese documentation lives in `.zh-CN.md` files. Existing `.en.md` URLs point readers to the canonical English pages. Published tags and distribution assets retain their original contents; documentation updates are delivered on the default branch and in subsequent packages.

## Prepare and verify

1. Update `internal/version/version.go`, `web/package.json` / lockfile, the Docker image version label, changelog and download links.
2. Build the UI with `make ui`. Run `go test ./...`, `go test -race ./...`, and `node --test web/test/query.test.mjs` in the appropriate environments.
3. Provision and test each supported database with `scripts/matrix.py`, then run `scripts/publish-verification.py`. Every passing report must match the current implementation digest. Keep only a dedicated PostgreSQL fixture for package checks: `MCPDBHUB_KEEP_FIXTURES=1 python3 scripts/matrix.py postgres`.
4. Refresh dependency notices when dependencies change. The Go collection uses `github.com/google/go-licenses/v2@v2.0.1`; preserve the reviewed MIT-0 / vendored Thrift supplements and native DuckDB notices, and inspect new unknown-license results. Frontend notices come from installed, locked packages. Collected source material stays under `third_party/licenses/_go` so it is not treated as application code by Go.
5. Capture UI changes through local Chrome using disposable data, update both README languages, and commit the release source.

## Build dist

```sh
python3 scripts/dist.py --version 0.1.0
# A single architecture, optionally using a trusted build-download CA:
python3 scripts/dist.py --version 0.1.0 --arch arm64 --build-ca /path/to/trusted-ca.pem
```

The script writes `dist/mcpdbhub-0.1.0-linux-{arm64,amd64}.tar.gz` and `dist/SHA256SUMS`. These archives require glibc 2.36 or newer. Keep the launcher, executable and private libraries together. `BUILD.json` includes the source commit, platform, binary hash and source image ID. On an arm64 machine, amd64 builds run through Docker's architecture support; do not describe emulation as native hardware validation.

## Validate the actual archives

With the retained, isolated PostgreSQL fixture available:

```sh
python3 scripts/check-package.py \
  --archive dist/mcpdbhub-0.1.0-linux-arm64.tar.gz \
  --report artifacts/release-0.1.0/dist-arm64.json
python3 scripts/check-package.py \
  --archive dist/mcpdbhub-0.1.0-linux-amd64.tar.gz \
  --report artifacts/release-0.1.0/dist-amd64.json
```

Each command unpacks into a fresh directory, verifies its manifest/hash, and runs its launcher inside a clean Debian container. It checks version, UID, runtime libraries, no Node runtime, bootstrap, HTTP/stdio MCP discovery and PostgreSQL queries, restart persistence, revocation and password recovery. It cleans up its own container, volume and extraction directory. `--image mcpdbhub:0.1.0-arm64` verifies an image instead of an archive.

## Publish

Push the reviewed commit to `main` and require both CI architecture jobs to pass. Create an annotated `v0.1.0` tag at that commit. Create the GitHub Release as a draft, attach both archives, `SHA256SUMS` and the actual validation records, then download those assets again and compare hashes. Verify the local, remote branch and dereferenced tag commits before publishing the Release as latest.

Do not attach configuration databases, master keys, fixture manifests, tokens, local logs or build caches. Do not move a published tag to another commit. Keep historical verification records tied to the source they actually tested.
