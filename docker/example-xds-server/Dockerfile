FROM golang:1.19.2-alpine as builder

RUN apk add --no-cache --update alpine-sdk \
    git \
    make

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY main.go ./

RUN go mod download

RUN go build -race -o example-xds-server main.go

FROM alpine as final

EXPOSE 18000

COPY --from=builder /app/example-xds-server /bin/

CMD ["/bin/example-xds-server"]
