FROM golang:alpine AS builder
WORKDIR /go/src/github.com/k8-proxy/go-k8s-process
COPY . .
RUN  env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o  go-k8s-process ./cmd

RUN apk update
RUN apk add git

FROM ubuntu:18.04


RUN mkdir -p /app
RUN mkdir -p /dep
COPY --from=builder /go/src/github.com/k8-proxy/go-k8s-process/go-k8s-process /app/go-k8s-process
COPY --from=builder /go/src/github.com/k8-proxy/go-k8s-process/dep/config.ini /dep/config.ini
COPY --from=builder /go/src/github.com/k8-proxy/go-k8s-process/dep/config.xml /dep/config.xml
COPY --from=builder /go/src/github.com/k8-proxy/go-k8s-process/sdk-rebuild-eval/libs/rebuild/linux/libglasswall.classic.so /usr/lib/libglasswall.classic.so
COPY --from=builder /go/src/github.com/k8-proxy/go-k8s-process/sdk-rebuild-eval/tools/command.line.tool/linux/glasswallCLI /dep/glasswallCLI
COPY --from=builder /go/src/github.com/k8-proxy/go-k8s-process/dep/glasswall.classic.conf /etc/ld.so.conf.d/glasswall.classic.conf

RUN apt-get update
RUN apt-get -y install  libfreetype6

RUN chmod +x /dep/glasswallCLI
ENV IN_CONTAINER=true       
ENV GWCLI=/dep/glasswallCLI         
ENV INICONFIG=/dep/config.ini   
ENV XMLCONFIG=/dep/config.xml

ENTRYPOINT ["bash","-c","/app/go-k8s-process"]

