FROM golang:alpine3.22 as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o relay .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/relay .
COPY --from=builder /app/config.example.json ./config.json

EXPOSE 3334
CMD ["./relay"]