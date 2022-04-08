FROM golang:1.18 AS builder

WORKDIR /app

COPY go.mod go.sum config.json ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/boetea

FROM scratch

WORKDIR /root/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/config.json .
COPY --from=builder /app/main .
CMD ["./main"]
