FROM golang:1.19.2-alpine as builder

RUN apk add --no-cache --update alpine-sdk \
    git \
    make

WORKDIR /app

COPY go.mod ./
COPY main.go ./
RUN go mod download

RUN go build -race -o example-upstream-server main.go

FROM alpine as final

EXPOSE 80

COPY --from=builder /app/example-upstream-server /bin/

CMD ["/bin/example-upstream-server"]
