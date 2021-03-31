FROM golang:alpine AS builder
WORKDIR /go/src/github.com/k8-proxy/go-k8s-process
COPY . .
RUN cd cmd \
    && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o  go-k8s-process .

FROM alpine
COPY --from=builder /go/src/github.com/k8-proxy/go-k8s-process/cmd/go-k8s-process /bin/go-k8s-process

RUN apk update && apk add ca-certificates

ENTRYPOINT ["/bin/go-k8s-process"]
