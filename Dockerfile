FROM golang:1.21-alpine3.18 AS builder

WORKDIR /build

COPY go.* ./
RUN go mod download

COPY . .
RUN go build -o /bin/www .

FROM alpine:3.18

COPY --from=builder /bin/www /bin/

ENTRYPOINT ["/bin/www", "start"]
