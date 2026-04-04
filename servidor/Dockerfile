FROM golang:1.20-alpine

WORKDIR /app

# Instala dependências primeiro (otimiza cache do Docker)
COPY go.mod ./
# Se você tiver um go.sum, inclua-o: COPY go.mod go.sum ./
RUN go mod download

# Copia o código fonte
COPY *.go ./

# Compila o binário
RUN go build -o servidor .

# Portas: 8080 (TCP/Atuador), 8081 (HTTP/API), 8082 (UDP/Sensor)
EXPOSE 8080 8081 8082/udp

CMD ["./servidor"]
