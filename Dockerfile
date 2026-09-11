FROM --platform=$BUILDPLATFORM node:24-bookworm AS ui
WORKDIR /src/web
COPY web/package*.json ./
RUN --mount=type=secret,id=build_ca \
    if [ -f /run/secrets/build_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/build_ca; fi; \
    npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.26-bookworm AS build
ARG VCS_REF=development
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
COPY --from=ui /src/internal/ui/dist ./internal/ui/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=build_ca \
    if [ -f /run/secrets/build_ca ]; then export SSL_CERT_FILE=/run/secrets/build_ca; fi; \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -X github.com/SamuelSupe/mcpdbhub/internal/version.Commit=$VCS_REF" -o /out/mcpdbhub ./cmd/mcpdbhub \
    && mkdir /runtime-libs \
    && cp -L "$(gcc -print-file-name=libstdc++.so.6)" /runtime-libs/ \
    && cp -L "$(gcc -print-file-name=libgcc_s.so.1)" /runtime-libs/ \
    && mkdir /runtime-licenses \
    && cp -L /usr/share/doc/libstdc++6/copyright /runtime-licenses/libstdc++6-copyright \
    && cp -L /usr/share/doc/libgcc-s1/copyright /runtime-licenses/libgcc-s1-copyright \
    && cp /usr/share/common-licenses/GPL-3 /usr/share/common-licenses/LGPL-3 /runtime-licenses/

FROM debian:bookworm-slim
ARG VCS_REF=development
LABEL org.opencontainers.image.title="MCP DB Hub" \
      org.opencontainers.image.description="Native read-only database access for AI agents" \
      org.opencontainers.image.source="https://github.com/SamuelSupe/mcpdbhub" \
      org.opencontainers.image.version="0.2.0" \
      org.opencontainers.image.revision=$VCS_REF
COPY --from=build /runtime-libs/ /usr/local/lib/
COPY --from=build /runtime-licenses/ /usr/share/doc/mcpdbhub-runtime/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/mcpdbhub /usr/local/bin/mcpdbhub
RUN mkdir -p /data /databases && chown 10001:10001 /data /databases
ENV LD_LIBRARY_PATH=/usr/local/lib \
    MCPDBHUB_LISTEN=0.0.0.0:8080 \
    MCPDBHUB_DATA_DIR=/data \
    MCPDBHUB_DATABASE_DIR=/databases
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["mcpdbhub"]
CMD ["serve"]
