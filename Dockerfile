FROM golang:1.21-alpine3.18 AS builder

WORKDIR /build

COPY go.* ./
RUN go mod download

COPY . .
RUN apk add make
RUN make www

FROM node:21.1-alpine3.18 AS assets

WORKDIR /build

COPY . .
RUN apk add make perl-utils
RUN make assets

FROM alpine:3.18

WORKDIR /app

COPY --from=builder /build/www .
COPY --from=assets /build/internal/www/public/ internal/www/public/

ENTRYPOINT ["/app/www", "start"]
