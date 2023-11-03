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
RUN apk add make
RUN make internal/www/public/styles/index.css

FROM alpine:3.18

COPY --from=builder /build/www /bin/
COPY --from=assets /build/internal/www/public/ internal/www/public/

ENTRYPOINT ["/bin/www", "start"]
