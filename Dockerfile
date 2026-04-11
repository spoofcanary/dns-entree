# syntax=docker/dockerfile:1
FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG TARGETOS TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${DATE}" \
    -o /out/entree ./cmd/entree

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/entree-api ./cmd/entree-api

FROM gcr.io/distroless/static-debian12

COPY --from=build /out/entree /entree
COPY --from=build /out/entree-api /entree-api

EXPOSE 8080

ENTRYPOINT ["/entree-api"]
CMD ["--listen", ":8080"]

LABEL org.opencontainers.image.source="https://github.com/spoofcanary/dns-entree"
LABEL org.opencontainers.image.description="dns-entree - DNS record generator and migration engine"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.licenses="MIT"
