FROM golang:1.25.5-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest

ENV SALT=53e6f8ea8e14a60ada82afaaa9721b584ebcb7df6a4e55250342bc088141cffa

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]