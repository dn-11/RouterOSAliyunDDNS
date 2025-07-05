FROM golang:1.22-alpine AS builder

WORKDIR /app

ADD . .

ENV CGO_ENABLED=0

RUN go build -o ddns

FROM alpine:3.12

COPY --from=builder /app/ddns /app/ddns

CMD ["/app/ddns"]
