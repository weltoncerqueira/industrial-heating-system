FROM golang:1.21-alpine

WORKDIR /app

# Copiar todos os arquivos Go
COPY . .

# Compilar o cliente
RUN go build -o cliente .

# Cliente não expõe portas (apenas conecta)
# O cliente é interativo, precisa de TTY

ENTRYPOINT ["./cliente"]
