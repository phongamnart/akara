FROM golang:latest

WORKDIR /app/register

COPY register.go .
COPY go.mod .
COPY go.sum .
COPY index.html .

RUN go build -o register .

CMD ["./register"]
