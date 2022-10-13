FROM golang:1.19.2-alpine as builder

RUN apk add --no-cache --update alpine-sdk \
    git \
    make

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

# see https://github.com/envoyproxy/go-control-plane/blob/v0.10.3/Makefile#L88
RUN cd /go/pkg/mod/github.com/envoyproxy/go-control-plane@v0.10.3 && \
    go build -race -o /app/example-xds-server internal/example/main/main.go

FROM alpine as final

EXPOSE 18000

COPY --from=builder /app/example-xds-server /bin/

CMD ["/bin/example-xds-server"]
