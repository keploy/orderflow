FROM golang:1.21-bookworm AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN rm -f go.sum && go mod tidy && CGO_ENABLED=1 go build -o consumer ./main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/consumer .
CMD ["./consumer"]