FROM golang:1.22-alpine AS builder

RUN set -eux; \
    apk update && apk add git

ENV GOPATH /go
ENV GO_WORKDIR $GOPATH/src/project-chimera/s1-exchange
ENV GOFLAGS=-mod=vendor
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR $GO_WORKDIR
ADD . $GO_WORKDIR
RUN go build -ldflags "-X 'main.GitCommitNum=`git rev-parse --short=12 HEAD`' -o s1-exchange ."

FROM asia-east1-docker.pkg.dev/cyr-gcp/cyr1/centos:7
ENV GOPATH /go
ENV GO_WORKDIR $GOPATH/src/project-chimera/s1-exchange
WORKDIR /app
COPY --from=builder $GO_WORKDIR/s1-exchange s1-exchange
COPY env.yaml .
COPY config.yaml .

EXPOSE 8081
CMD ["./s1-exchange"]


