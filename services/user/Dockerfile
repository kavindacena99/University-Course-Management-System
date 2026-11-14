FROM golang:1.27.2-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user-service .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user-migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user-bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
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
