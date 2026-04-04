FROM golang:1.20-alpine

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY *.go ./

# Compila o binário do atuador
RUN go build -o atuador .

CMD ["./atuador"]