# syntax=docker/dockerfile:1
# Lenapay API: one image, four binaries (api, worker, migrate, seed).
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOWORK=off
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
 && go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.22
RUN adduser -D -u 10001 lenapay
COPY --from=build /out/ /app/
USER lenapay
EXPOSE 8080
ENTRYPOINT ["/app/api"]
