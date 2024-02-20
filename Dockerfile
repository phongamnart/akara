FROM golang:latest

WORKDIR /app

COPY go.mod go.sum .
COPY register.go .
COPY index.html .

RUN go build -o register .

CMD ["./register"]