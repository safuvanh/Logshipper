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
FROM alpine:3.23.3
RUN apk add --no-cache ca-certificates bash && \
    addgroup -g 1000 -S logshipper && adduser -u 1000 -S logshipper -G logshipper
COPY --from=build /app /usr/local/bin/logshipper
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/logshipper"]
