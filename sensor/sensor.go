package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// =====================
// ESTRUTURAS
// =====================

type Dispositivo struct {
	ID                string    `json:"id"`
	Nome              string    `json:"nome"`
	Tipo              string    `json:"tipo"`
	Temperatura       float64   `json:"temperatura"`
	TemperaturaAlvo   float64   `json:"temperatura_alvo"`
	Status            string    `json:"status"`
	UltimaAtualizacao time.Time `json:"ultima_atualizacao"`
}

type Mensagem struct {
	Tipo     string          `json:"tipo"`
	De       string          `json:"de"`
	Para     string          `json:"para"`
	Conteudo json.RawMessage `json:"conteudo"`
}

var (
	dispositivo Dispositivo
	mu          sync.Mutex
)

// =====================
// MAIN
// =====================

func main() {
	idFlag := flag.String("id", "sensor-01", "ID do sensor")
	portaFlag := flag.String("porta", "8083", "Porta UDP para escuta")
	hostFlag := flag.String("host", "127.0.0.1", "IP do servidor")

	flag.Parse()
	dispositivo = Dispositivo{
		ID:              *idFlag,
		Nome:            fmt.Sprintf("Sensor Térmico %s", *idFlag),
		Tipo:            "sensor",
		Temperatura:     25.0,
		TemperaturaAlvo: 0.0,
		Status:          "ativo",
	}

	fmt.Println("========================================")
	fmt.Printf("🚀 SENSOR %s INICIADO\n", dispositivo.ID)
	fmt.Println("========================================")
	fmt.Printf("📡 Servidor UDP: %s:8082\n", *hostFlag)
	fmt.Printf("👂 Escutando comandos em: %s\n", *portaFlag)

	go loopSimulacaoEEnvio(*hostFlag)
	go escutarComandosServidor(*portaFlag)

	select {}
}

// =====================
// ENVIO DE DADOS
// =====================

// =====================
// ENVIO DE DADOS (COM SIMULAÇÃO DE QUEDA REALISTA)
// =====================

func loopSimulacaoEEnvio(hostServidor string) {
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:8082", hostServidor))
	if err != nil {
		fmt.Println("❌ Erro ao resolver servidor:", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Println("❌ Erro ao conectar UDP:", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(2 * time.Second)
	tempAmbiente := 25.0

	for range ticker.C {
		mu.Lock()

		// 1. Ruído constante (pequena oscilação natural de 0.1°C)
		dispositivo.Temperatura += (rand.Float64() - 0.5) * 0.1

		// 2. Lógica de Simulação Física
		if dispositivo.TemperaturaAlvo > 0 {
			// AQUECIMENTO ATIVO: O atuador está ligado e buscando o alvo
			diff := dispositivo.TemperaturaAlvo - dispositivo.Temperatura
			// Sobe ou desce em direção ao alvo (passo de 10% da diferença)
			dispositivo.Temperatura += diff * 0.1
			dispositivo.Status = "aquecendo" // Status simples para aquecimento
		} else {
			// RESFRIAMENTO PASSIVO (Atuador Desligado ou Alvo 0)
			if dispositivo.Temperatura > tempAmbiente+0.5 {
				// Lei do resfriamento: perde 5% da diferença para o ambiente por ciclo
				perda := (dispositivo.Temperatura - tempAmbiente) * 0.05

				// Garante uma queda mínima de 0.5°C para não ficar lento demais no final
				if perda < 0.5 {
					perda = 0.5
				}
				dispositivo.Temperatura -= perda
				dispositivo.Status = "resfriando" // Status para resfriamento
			} else {
				// Estabiliza na temperatura ambiente com leve oscilação
				dispositivo.Temperatura = tempAmbiente + (rand.Float64() * 0.4)
				dispositivo.Status = "ocioso" // Status quando está em temperatura ambiente
			}
		}

		dispositivo.UltimaAtualizacao = time.Now()

		// 3. Empacotamento e Envio
		data, _ := json.Marshal(dispositivo)
		msg := Mensagem{
			Tipo:     "dados_sensor",
			De:       dispositivo.ID,
			Para:     "servidor",
			Conteudo: data,
		}
		mu.Unlock()

		final, _ := json.Marshal(msg)
		_, err := conn.Write(final)

		if err != nil {
			fmt.Println("⚠️ Erro ao enviar:", err)
		} else {
			fmt.Printf("📊 Temp: %.2f°C | Alvo: %.1f°C | Status: %s\n",
				dispositivo.Temperatura,
				dispositivo.TemperaturaAlvo,
				dispositivo.Status)
		}
	}
}

// =====================
// RECEBER COMANDOS
// =====================
func escutarComandosServidor(porta string) {
	localAddr, err := net.ResolveUDPAddr("udp", ":"+porta)
	if err != nil {
		fmt.Println("❌ Erro ao resolver porta:", err)
		return
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		fmt.Printf("❌ Porta %s em uso! (%v)\n", porta, err)
		return
	}
	defer conn.Close()

	fmt.Println("✅ Escuta UDP ativa!")

	buf := make([]byte, 2048)

	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		var msg Mensagem
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			continue
		}

		if msg.Tipo != "ajustar_simulacao" {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Conteudo, &payload); err != nil {
			fmt.Println("⚠️ Erro ao decodificar payload:", err)
			continue
		}

		//Verificar se é comando de resfriamento natural
		if cmd, exists := payload["comando"].(string); exists && cmd == "resfriar_natural" {
			mu.Lock()
			dispositivo.TemperaturaAlvo = 0.0
			fmt.Printf("🌡️ RESFRIAMENTO NATURAL iniciado (alvo=0) - temperatura atual: %.2f°C\n",
				dispositivo.Temperatura)
			mu.Unlock()
			continue
		}

		// Processar temperatura alvo normalmente
		var novoAlvo float64
		var ok bool

		if v, exists := payload["temperatura_alvo"].(float64); exists {
			novoAlvo = v
			ok = true
		}

		if !ok {
			fmt.Println("⚠️ Comando recebido sem temperatura válida")
			continue
		}

		//Verifico se os dados são os mesmos. Em caso negativo, eu atualizo
		mu.Lock()
		if dispositivo.TemperaturaAlvo != novoAlvo {
			dispositivo.TemperaturaAlvo = novoAlvo
			if novoAlvo == 0.0 {
				fmt.Printf("🌡️ AQUECIMENTO DESLIGADO (alvo=0) - resfriamento natural em %s\n",
					addr.IP.String())
			} else {
				fmt.Printf("🌡️ NOVO ALVO RECEBIDO de %s: %.1f°C\n", addr.IP.String(), novoAlvo)
			}
		}
		mu.Unlock()
	}
}
