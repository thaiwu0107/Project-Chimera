FROM golang:1.22-alpine AS builder

RUN set -eux; \
    apk update && apk add git

ENV GOPATH /go
ENV GO_WORKDIR $GOPATH/src/project-chimera/s4-router
ENV GOFLAGS=-mod=vendor
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR $GO_WORKDIR
ADD . $GO_WORKDIR
RUN go build -ldflags "-X 'main.GitCommitNum=git rev-parse --short=12 HEAD' -o s4-router ."

FROM asia-east1-docker.pkg.dev/cyr-gcp/cyr1/centos:7
ENV GOPATH /go
ENV GO_WORKDIR $GOPATH/src/project-chimera/s4-router
WORKDIR /app
COPY --from=builder $GO_WORKDIR/s4-router s4-router
COPY env.yaml .
COPY config.yaml .

EXPOSE 8084
CMD ["./s4-router"]


