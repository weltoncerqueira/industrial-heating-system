package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// Estruturas de Dados (Sincronizadas com o Servidor)
type Dispositivo struct {
	ID                string    `json:"id"`
	Nome              string    `json:"nome"`
	Tipo              string    `json:"tipo"`
	Status            string    `json:"status"`
	TemperaturaAlvo   float64   `json:"temperatura_alvo"`
	UltimaAtualizacao time.Time `json:"ultima_atualizacao"`
	SensorPareadoID   string    `json:"sensor_pareado_id"`
}

type Mensagem struct {
	Tipo     string          `json:"tipo"`
	De       string          `json:"de"`
	Para     string          `json:"para"`
	Conteudo json.RawMessage `json:"conteudo"`
}

var (
	dispositivo Dispositivo
	conexao     net.Conn
	mu          sync.Mutex
)

func main() {
	// 1. Configuração via Variáveis de Ambiente
	// Se você não passar nada no terminal, ele usa "atuador-01", etc.
	idFlag := flag.String("id", "atuador-01", "ID do atuador")
	nomeFlag := flag.String("nome", "Aquecedor Industrial", "Nome do dispositivo")
	parearFlag := flag.String("parear", "sensor-01", "ID do sensor pareado")
	hostFlag := flag.String("host", "127.0.0.1", "IP do servidor") //colocar o ip do servidor aqui =================

	flag.Parse()

	// Agora preenchemos a struct DIRETAMENTE com os ponteiros das flags
	dispositivo = Dispositivo{
		ID:                *idFlag,
		Nome:              *nomeFlag,
		Tipo:              "atuador",
		Status:            "desligado",
		TemperaturaAlvo:   0.0,
		UltimaAtualizacao: time.Now(),
		SensorPareadoID:   *parearFlag,
	}

	fmt.Printf("\n🔥 ATUADOR [%s] - %s\n", dispositivo.ID, dispositivo.Nome)
	fmt.Printf("🔗 Pareado com: %s | Servidor: %s:8080\n", dispositivo.SensorPareadoID, *hostFlag)

	conectarAoServidor(*hostFlag)
}

func conectarAoServidor(host string) {
	endereco := fmt.Sprintf("%s:8080", host)

	for {
		conn, err := net.DialTimeout("tcp", endereco, 5*time.Second)
		if err != nil {
			fmt.Printf("❌ Falha na conexão: %v. Nova tentativa em 5s...\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		mu.Lock()
		conexao = conn
		mu.Unlock()

		fmt.Println("✅ Conectado ao Servidor TCP!")

		registrarDispositivo()
		receberComandos() // Bloqueia aqui até a conexão cair

		fmt.Println("⚠️ Conexão perdida. Tentando reconectar...")
		mu.Lock()
		conexao = nil
		mu.Unlock()
		time.Sleep(2 * time.Second)
	}
}

func registrarDispositivo() {
	mu.Lock()
	dados, _ := json.Marshal(dispositivo)
	msg := Mensagem{
		Tipo:     "registrar",
		De:       dispositivo.ID,
		Para:     "servidor",
		Conteudo: dados,
	}
	mu.Unlock()
	enviarJSON(msg)
}

func receberComandos() {
	// scanner lê linha a linha (o servidor envia JSON + \n)
	scanner := bufio.NewScanner(conexao)
	for scanner.Scan() {
		var msg Mensagem
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Tipo == "comando" {
			processarComando(msg.Conteudo)
		}
	}
}

func processarComando(conteudo json.RawMessage) {
	var dados map[string]interface{}
	if err := json.Unmarshal(conteudo, &dados); err != nil {
		return
	}

	mu.Lock()
	// Mapeia os dados recebidos do comando para o estado interno
	if status, ok := dados["status"].(string); ok {
		dispositivo.Status = status
	}
	if temp, ok := dados["target_temperature"].(float64); ok {
		dispositivo.TemperaturaAlvo = temp
	}
	dispositivo.UltimaAtualizacao = time.Now()

	fmt.Printf("📥 COMANDO: Status=%s | Alvo=%.1f°C\n", dispositivo.Status, dispositivo.TemperaturaAlvo)
	mu.Unlock()

	// Devolve o status atualizado para o servidor manter o Dashboard correto
	atualizarStatusNoServidor()
}

func atualizarStatusNoServidor() {
	mu.Lock()
	dados, _ := json.Marshal(dispositivo)
	msg := Mensagem{
		Tipo:     "dados_atuador",
		De:       dispositivo.ID,
		Para:     "servidor",
		Conteudo: dados,
	}
	mu.Unlock()
	enviarJSON(msg)
}

func enviarJSON(msg Mensagem) {
	mu.Lock()
	defer mu.Unlock()

	if conexao == nil {
		return
	}

	dados, _ := json.Marshal(msg)
	// Adiciona \n para o servidor saber onde termina o JSON (bufio.Scanner)
	_, err := fmt.Fprintf(conexao, "%s\n", dados)
	if err != nil {
		fmt.Printf("⚠️ Erro ao enviar dados: %v\n", err)
	}
}

// Helper para variáveis de ambiente
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
