FROM golang:1.22-alpine3.12 as builder

WORKDIR /app

ADD * /app

ENV GOOS=linux
ENV CGO_ENABLED=0

RUN cd /app && go build -o /app/main

FROM alpine:3.12

COPY --from=builder /app/main /app/main

CMD ["/app/main"]

