FROM golang:1.20-alpine AS builder

COPY ./ /app
WORKDIR /app
RUN go get ./...
RUN go build -o ./server ./example/server/main.go

FROM alpine
COPY --from=builder /app/server /server
CMD ["/server"]
