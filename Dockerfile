FROM golang:latest

WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy
RUN go build -o main ./cmd/kgs

CMD ["./main"]
