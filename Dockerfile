FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache libc6-compat
WORKDIR /app
COPY --from=builder /app/server .
RUN mkdir -p /app/data
VOLUME ["/app/data"]
EXPOSE 8080
CMD ["./server"]
