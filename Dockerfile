FROM golang:1.27 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/boetea ./cmd/boetea
# scratch has no directories. Media spooling and heap snapshots write to /tmp.
RUN mkdir -p /out/tmp && chmod 1777 /out/tmp && cp quotes.json /out/

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/ /
USER 65532:65532
ENV BOETEA_QUOTES_FILE=/quotes.json
ENTRYPOINT ["/boetea"]
