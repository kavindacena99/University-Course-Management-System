FROM golang:1.27.2-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/analytics-service .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/analytics-migrate ./cmd/migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=build /out/analytics-service /app/analytics-service
COPY --from=build /out/analytics-migrate /app/analytics-migrate
COPY --from=build /src/migrations /app/migrations

USER app

EXPOSE 8086

ENTRYPOINT ["/app/analytics-service"]
