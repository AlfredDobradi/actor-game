FROM golang:buster AS build
COPY . /build
WORKDIR /build
RUN go build -o target/ ./cmd/...

FROM ubuntu:20.04
COPY --from=build /build/target/gamed /usr/bin/gamed
ENTRYPOINT /usr/bin/gamed
