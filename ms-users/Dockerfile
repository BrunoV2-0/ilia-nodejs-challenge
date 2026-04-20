FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ms-users ./cmd/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/ms-users .
COPY --from=builder /app/internal/database/migrations ./internal/database/migrations
EXPOSE 3002
CMD ["./ms-users"]
