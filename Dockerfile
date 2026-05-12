# This image is for CI builds. Run radar natively in development.

FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /radar ./cmd/radar

FROM scratch
COPY --from=builder /radar /radar
ENTRYPOINT ["/radar"]
