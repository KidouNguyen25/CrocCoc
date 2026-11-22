# NOTE: This Linux container verifies frontend tests/build and platform-neutral
# Go packages (!windows) only. Go build tags exclude all *_windows.go files on
# Linux (Win32 protected clipboard, Job Object, NTFS handle leases, staging and
# no-replace publish), so passing in this container never replaces native
# verification on a Windows amd64 host (see docs/verification.md).
FROM node:24-bookworm

ARG GO_VERSION=1.26.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
        | tar -C /usr/local -xz \
    && apt-get purge -y --auto-remove curl \
    && rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN npm --prefix frontend ci

COPY . .

CMD ["sh", "-c", "npm --prefix frontend test && npm --prefix frontend run build && go test ./... && go vet ./..."]
