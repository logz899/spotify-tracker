FROM golang:1.26.8-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/sync ./cmd/sync

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/migrate .
COPY --from=builder /app/sync .
COPY migrations/ ./migrations/

EXPOSE 8000

CMD ["./server"]
