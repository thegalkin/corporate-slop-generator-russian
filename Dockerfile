# Сборка: нужен только Go, словарь уже вшит в бинарник.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cbsg ./cmd/cbsg

FROM alpine:3.20
RUN adduser -D -u 10001 slop
COPY --from=build /out/cbsg /usr/local/bin/cbsg
USER slop
ENTRYPOINT ["/usr/local/bin/cbsg"]
CMD ["-n", "10"]
