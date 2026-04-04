FROM golang:1.20-alpine

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY *.go ./

# Compila o binário do sensor
RUN go build -o sensor .

# Porta onde o sensor ouve comandos do servidor
EXPOSE 8083/udp

CMD ["./sensor"]