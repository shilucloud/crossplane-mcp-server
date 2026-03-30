FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o crossplane-agent ./cmd/server

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/crossplane-agent .
ENTRYPOINT ["./crossplane-agent", "--http", "0.0.0.0:8080"]