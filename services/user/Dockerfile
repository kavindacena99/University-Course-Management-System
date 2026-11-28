FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod --mount=type=cache,id=ucm-go-build,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user-service .
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod --mount=type=cache,id=ucm-go-build,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user-migrate ./cmd/migrate
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod --mount=type=cache,id=ucm-go-build,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user-bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:3.22@sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f
RUN apk add --no-cache ca-certificates 'libcrypto3>=3.5.9-r2' 'libssl3>=3.5.9-r2' \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/user-service /app/user-service
COPY --from=build /out/user-migrate /app/user-migrate
COPY --from=build /out/user-bootstrap-admin /app/user-bootstrap-admin
COPY --from=build /src/migrations /app/migrations

USER app
EXPOSE 8082
ENTRYPOINT ["/app/user-service"]
