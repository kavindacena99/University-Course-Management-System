# Same PostgreSQL 18.7 server/data format; rebuild upstream gosu 1.20 only.
FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS helper
WORKDIR /src
ADD --checksum=sha256:2b70da8a5480fafedc16bb1fc91b572f064d03df736cbd91a616af2f1b66ab51 https://codeload.github.com/tianon/gosu/tar.gz/29b939aa5bd4dc588af765b8248f6e39150d2565 /tmp/gosu.tar.gz
RUN tar -xzf /tmp/gosu.tar.gz --strip-components=1 -C /src \
    && go mod download && go mod verify \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/gosu .

FROM postgres:18-alpine@sha256:0115267ab5a509022df9319c77be365cef335adb71556c75f60f5360ee247e71
RUN apk add --no-cache 'libxml2>=2.13.9-r3'
COPY --from=helper --chmod=0755 /out/gosu /usr/local/bin/gosu
LABEL org.opencontainers.image.description="PostgreSQL 18.7 with unchanged upstream gosu 1.20 rebuilt on Go 1.27.3" \
      org.opencontainers.image.source="https://github.com/tianon/gosu" \
      org.opencontainers.image.revision="29b939aa5bd4dc588af765b8248f6e39150d2565"
