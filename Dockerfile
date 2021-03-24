FROM golang:alpine AS builder
WORKDIR /go/src/github.com/k8-proxy/icap-service2
COPY . .
RUN cd cmd \
    && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o  icap-service2 .

FROM alpine
COPY --from=builder /go/src/github.com/k8-proxy/icap-service2/cmd/icap-service2 /bin/icap-service2

RUN apk update && apk add ca-certificates

ENTRYPOINT ["/bin/icap-service2"]
