FROM golang:latest

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG SERVICE_NAME=kgs
RUN go build -o main ./cmd/${SERVICE_NAME}

CMD ["./main"]
