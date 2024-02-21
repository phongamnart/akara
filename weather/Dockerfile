FROM golang:latest

WORKDIR /app/weather

COPY weather.go .
COPY go.mod .
COPY go.sum .

RUN go build -o weather .

CMD ["./weather"]

