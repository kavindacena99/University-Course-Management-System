FROM golang:1.27.3-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod --mount=type=cache,id=ucm-go-build,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/assessment-service .
RUN --mount=type=cache,id=ucm-go-mod,target=/go/pkg/mod --mount=type=cache,id=ucm-go-build,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/assessment-migrate ./cmd/migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=build /out/assessment-service /app/assessment-service
COPY --from=build /out/assessment-migrate /app/assessment-migrate
COPY --from=build /src/migrations /app/migrations

USER app

EXPOSE 8085

ENTRYPOINT ["/app/assessment-service"]
