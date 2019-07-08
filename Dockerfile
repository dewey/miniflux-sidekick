FROM golang:1.12-alpine as builder

RUN apk add git bash

ENV GO111MODULE=on

# Add our code
ADD ./ $GOPATH/src/github.com/dewey/miniflux-sidekick

# build
WORKDIR $GOPATH/src/github.com/dewey/miniflux-sidekick
RUN cd $GOPATH/src/github.com/dewey/miniflux-sidekick && \    
    GO111MODULE=on GOGC=off go build -mod=vendor -v -o /miniflux-sidekick ./cmd/api/

# multistage
FROM alpine:latest

# https://stackoverflow.com/questions/33353532/does-alpine-linux-handle-certs-differently-than-busybox#33353762
RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

COPY --from=builder /miniflux-sidekick /usr/bin/miniflux-sidekick

# Run the image as a non-root user
RUN adduser -D mfs
RUN chmod 0755 /usr/bin/miniflux-sidekick

USER mfs

# Run the app. CMD is required to run on Heroku
CMD miniflux-sidekick 