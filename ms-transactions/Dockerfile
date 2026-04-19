FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ms-transactions ./cmd/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/ms-transactions .
EXPOSE 3001
CMD ["./ms-transactions"]
