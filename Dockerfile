# Build stage
FROM golang:1.21-alpine AS build
ENV GOPROXY=https://proxy.golang.org,direct
RUN apk add --no-cache git ca-certificates bash
WORKDIR /src

# copy module files first (helps cache) and all source
COPY go.mod  ./
COPY cmd cmd
COPY internal internal

# tidy and download deps
RUN go mod tidy
RUN go mod download

# build binary from cmd entrypoint
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /app ./cmd/logshipper

# Runtime stage
FROM alpine:3.22.2
RUN apk add --no-cache ca-certificates bash
COPY --from=build /app /usr/local/bin/logshipper
USER root
ENTRYPOINT ["/usr/local/bin/logshipper"]
