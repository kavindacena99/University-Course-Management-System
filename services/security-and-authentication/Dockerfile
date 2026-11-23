FROM golang:1.27.3-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod --mount=type=cache,id=ucm-go-build,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth-service .
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod --mount=type=cache,id=ucm-go-build,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth-migrate ./cmd/migrate

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/var \
    && chown app:app /app/var

WORKDIR /app
COPY --from=build /out/auth-service /app/auth-service
COPY --from=build /out/auth-migrate /app/auth-migrate
COPY --from=build /src/migrations /app/migrations

USER app
EXPOSE 8081
ENTRYPOINT ["/app/auth-service"]
