# builder
FROM golang:1.22 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o mongodb-index-advisor .
RUN ls -lh /app/mongodb-index-advisor

# runner
FROM debian:stable

WORKDIR /app
COPY --from=builder /usr/share/zoneinfo /usr/share/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/mongodb-index-advisor .
ENTRYPOINT ["/app/mongodb-index-advisor"]
CMD ["-h"]
