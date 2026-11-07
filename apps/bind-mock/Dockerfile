# syntax=docker/dockerfile:1
# Fake BIND PSP for local runs (never deployed).
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
ENV CGO_ENABLED=0 GOWORK=off
RUN go build -trimpath -ldflags="-s -w" -o /out/bind-mock ./cmd/bind-mock

FROM alpine:3.22
RUN adduser -D -u 10001 bindmock && mkdir /data && chown bindmock /data
COPY --from=build /out/bind-mock /app/bind-mock
USER bindmock
EXPOSE 8090
ENTRYPOINT ["/app/bind-mock"]
