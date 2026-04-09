# Adicionar no docker-compose.yaml do cliente:
services:
  cliente:
    build: .
    container_name: industrial-cliente
    network_mode: host  # Já que os outros usam host network
    environment:
      - INTEGRATION_HOST=192.168.1.100  # IP do servidor
      - SERVER_PORT=8081                # Porta HTTP da API
    stdin_open: true
    tty: true
    command: ["-host=${INTEGRATION_HOST}", "-porta=${SERVER_PORT}"]
