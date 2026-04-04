# 🏭 Sistema de Controle de Aquecimento Industrial

Sistema distribuído para controle de aquecedores industriais utilizando Go. Desenvolvido para o PBL da disciplina de Redes de Computadores.

---

## 📋 Descrição

Este sistema simula o controle remoto de aquecedores industriais em um ambiente distribuído, permitindo:

* 🌡️ Monitorar temperatura em tempo real via sensor (UDP)
* 🔥 Controlar aquecedores via atuador (TCP)
* 🎯 Definir temperatura alvo remotamente
* 📊 Visualizar dispositivos via API HTTP
* 🎮 Interface interativa via cliente

---

## 📊 Estrutura do Projeto

```bash
industrial-heating-system/
├── README.md
├── servidor/
│   ├── main.go
│   └── go.mod
├── sensor/
│   ├── sensor.go
│   └── go.mod
├── atuador/
│   ├── atuador.go
│   └── go.mod
└── cliente/
    ├── main.go
    └── go.mod
```

---

## 🔌 Componentes

| Componente | Protocolo    | Porta              | Função            |
| ---------- | ------------ | ------------------ | ----------------- |
| Servidor   | TCP/UDP/HTTP | 8080 / 8081 / 8082 | Integração        |
| Sensor     | UDP          | 8082 / 8083        | Envia temperatura |
| Atuador    | TCP          | 8080               | Recebe comandos   |
| Cliente    | TCP/HTTP     | 8080 / 8081        | Interface         |

---

## 🔄 Fluxo de Dados

1. Sensor envia dados via UDP → servidor (8082)
2. Cliente envia comandos via TCP → servidor (8080)
3. Servidor encaminha para atuador
4. Atuador executa ação
5. Cliente consulta API HTTP → `/devices`

---

## 📋 Pré-requisitos

* Go instalado
* Computadores na mesma rede (ex: 172.30.x.x)
* Firewall liberado (ou desativado)

---

# 🚀 Execução no Laboratório

## 🔴 IMPORTANTE

* Use o IP do servidor: **172.30.44.155**
* Execute cada componente em um computador diferente

---

## 🖥️ SERVIDOR (PC 1)

```bash
cd industrial-heating-system
cd servidor
go run main.go
```

---

## 🌡️ SENSOR (PC 2)

```bash
cd industrial-heating-system
cd sensor
# Exemplo para o primeiro sensor
go run sensor.go -id="sensor-01" -host="172.30.44.155" -porta=":8083"
```

---

## 🔥 ATUADOR (PC 3)

```bash
cd industrial-heating-system
cd atuador
# Exemplo pareando com o sensor-01
go run atuador.go -id="atuador-01" -parear="sensor-01" -host="172.30.44.155"
```

---

## 🎮 CLIENTE (PC 4)

```bash
cd industrial-heating-system
cd cliente
# Configure o IP do servidor de integração
export INTEGRATION_HOST=172.30.44.155
go run main.go
```

---

# 🛡️ Segurança Industrial (Interlock)
O sistema possui uma camada de proteção nativa no Sensor:

Limite Crítico: 450°C.

#### Ação: Ao atingir o limite, o sensor força o TemperaturaAlvo = 0.0 e entra em modo de resfriamento natural, ignorando comandos externos até que a temperatura retorne a níveis seguros.

# 🔍 DEBUG DE REDE E FIREWALL

## TCP (Comandos/Dashboard)
nc -vz 172.30.44.155 8080
## HTTP (API)
nc -vz 172.30.44.155 8081
## UDP (Sensores)
nc -vzu 172.30.44.155 8082

## 🌐 Testar conectividade

```bash
ip a
ping -c 4 172.30.44.155
```

---

## 🔌 Testar portas

```bash
nc -vz 172.30.44.155 8080
nc -vz 172.30.44.155 8081
nc -vzu 172.30.44.155 8082
```

---

## 🔥 Firewall (Linux)

```bash
sudo ufw status verbose

sudo ufw allow 8080/tcp
sudo ufw allow 8081/tcp
sudo ufw allow 8082/udp
sudo ufw allow 8083/udp

sudo ufw reload

# (opcional)
sudo ufw disable
```

---

## 📡 Testar API

```bash
curl http://172.30.44.155:8081/devices
```

---

## 📊 Ver portas abertas

```bash
sudo ss -tulnp | grep 808
```

---

# 🛠️ Troubleshooting

### ❌ Cliente não conecta

* Verifique IP do servidor
* Teste com `ping`
* Teste porta 8080

---

### ❌ Sensor não aparece

* Verifique UDP 8082
* Verifique firewall

---

### ❌ Atuador não responde

* Verifique TCP 8080
* Verifique pareamento com sensor

---

### ❌ Nada funciona

```bash
sudo ufw disable
```

---

# ⚠️ Observações Importantes

* Não usar WSL (problemas de rede NAT)
* Servidor deve iniciar primeiro
* Todos na mesma rede
* Usar IP real (não usar localhost)

---

# 🧠 Fluxo Git

```bash
git status
git add .
git commit -m "update"
git pull
```

---

# 🚀 Resultado Esperado

* Sensor enviando temperatura
* Atuador respondendo comandos
* Cliente listando dispositivos
* Comunicação distribuída funcionando

---
