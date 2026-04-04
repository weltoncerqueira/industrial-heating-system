package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Estruturas Sincronizadas com o Servidor
type Dispositivo struct {
	ID                string    `json:"id"`
	Nome              string    `json:"nome"`
	Tipo              string    `json:"tipo"`
	Temperatura       float64   `json:"temperatura"`
	Status            string    `json:"status"`
	TemperaturaAlvo   float64   `json:"temperatura_alvo"`
	UltimaAtualizacao time.Time `json:"ultima_atualizacao"`
}

type Mensagem struct {
	Tipo     string          `json:"tipo"`
	De       string          `json:"de"`
	Para     string          `json:"para"`
	Conteudo json.RawMessage `json:"conteudo"`
}

type ClienteIntegracao struct {
	conexao net.Conn
	host    string
}

func main() {
	fmt.Println("========================================")
	fmt.Println("🎮 CLIENTE DE CONTROLE INDUSTRIAL v3.0")
	fmt.Println("========================================")

	host := os.Getenv("INTEGRATION_HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	c := &ClienteIntegracao{host: host}
	c.conectar()
	c.menu()
}

// Conexão TCP persistente com o Servidor (Porta 8080)
func (c *ClienteIntegracao) conectar() {
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:8080", c.host))
		if err != nil {
			fmt.Println("⏳ Aguardando servidor de comandos (8080)...")
			time.Sleep(2 * time.Second)
			continue
		}
		c.conexao = conn
		fmt.Println("✅ Conectado ao Barramento de Comandos!")
		return
	}
}

// Busca a lista de dispositivos via API HTTP (Porta 8081)
// Mantive o padrão HTTP pois o seu servidor já entrega o JSON pronto nela
func (c *ClienteIntegracao) buscarDispositivos() map[string]Dispositivo {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:8081", c.host), 2*time.Second)
	if err != nil {
		return nil
	}
	defer conn.Close()

	req := "GET /devices HTTP/1.1\r\nHost: integration\r\nConnection: close\r\n\r\n"
	conn.Write([]byte(req))

	// Lendo a resposta (simplificado para o contexto industrial)
	scanner := bufio.NewScanner(conn)
	var body string
	encontrouCorpo := false

	for scanner.Scan() {
		linha := scanner.Text()
		if linha == "" {
			encontrouCorpo = true
			continue
		}
		if encontrouCorpo {
			body += linha
		}
	}

	var d map[string]Dispositivo
	json.Unmarshal([]byte(body), &d)
	return d
}

func (c *ClienteIntegracao) menu() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println("\n╔════════════════════════════════════╗")
		fmt.Println("║         PAINEL DE CONTROLE         ║")
		fmt.Println("╠════════════════════════════════════╣")
		fmt.Println("║  1 - Listar Todos Dispositivos     ║")
		fmt.Println("║  2 - Controlar Atuador             ║")
		fmt.Println("║  3 - Sair                          ║")
		fmt.Println("╚════════════════════════════════════╝")
		fmt.Print("Escolha: ")

		scanner.Scan()
		op := scanner.Text()

		switch op {
		case "1":
			c.listar()
		case "2":
			c.selecionarAtuador()
		case "3":
			fmt.Println("👋 Encerrando...")
			return
		}
	}
}

func (c *ClienteIntegracao) listar() {
	dispositivos := c.buscarDispositivos()
	if dispositivos == nil {
		fmt.Println("❌ Erro ao obter dados do servidor.")
		return
	}

	fmt.Println("\n--- STATUS DO SISTEMA ---")
	for _, d := range dispositivos {
		info := ""
		if d.Tipo == "sensor" {
			info = fmt.Sprintf("%.2f°C", d.Temperatura)
		} else {
			info = fmt.Sprintf("[%s] Alvo: %.1f°C", d.Status, d.TemperaturaAlvo)
		}
		fmt.Printf("• %-12s | %-10s | %s\n", d.ID, d.Tipo, info)
	}
}

func (c *ClienteIntegracao) selecionarAtuador() {
	dispositivos := c.buscarDispositivos()
	var lista []Dispositivo
	for _, d := range dispositivos {
		if d.Tipo == "atuador" {
			lista = append(lista, d)
		}
	}

	if len(lista) == 0 {
		fmt.Println("⚠️ Nenhum atuador online.")
		return
	}

	fmt.Println("\n--- SELECIONE O ATUADOR ---")
	for i, a := range lista {
		fmt.Printf("%d) %s (%s)\n", i+1, a.Nome, a.ID)
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("ID: ")
	scanner.Scan()
	idx, _ := strconv.Atoi(scanner.Text())

	if idx < 1 || idx > len(lista) {
		fmt.Println("❌ Inválido.")
		return
	}

	c.operar(lista[idx-1])
}

func (c *ClienteIntegracao) operar(a Dispositivo) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("\n🎮 Controlando %s. Comandos: 'ligar', 'desligar', 'temp X', 'voltar'\n", a.Nome)

	for {
		fmt.Print("> ")
		scanner.Scan()
		input := scanner.Text()

		if input == "voltar" {
			return
		}

		payload := make(map[string]interface{})

		if input == "ligar" {
			payload["status"] = "ligado"
			payload["target_temperature"] = 100.0 // Define um alvo padrão ao ligar
		} else if input == "desligar" {
			payload["status"] = "desligado"
			payload["target_temperature"] = 0.0 // <--- ESSENCIAL para o Servidor ler e mandar o 0.0 pro Sensor
		} else if strings.HasPrefix(input, "temp ") {
			val, _ := strconv.ParseFloat(strings.TrimPrefix(input, "temp "), 64)
			payload["target_temperature"] = val
			// O campo "valor" é o que o seu Servidor usa para disparar o UDP pro sensor
			payload["valor"] = val
		} else {
			fmt.Println("Comando não reconhecido.")
			continue
		}

		c.enviarComando(a.ID, payload)
	}
}

func (c *ClienteIntegracao) enviarComando(idAlvo string, conteudo map[string]interface{}) {
	bytesConteudo, _ := json.Marshal(conteudo)

	msg := Mensagem{
		Tipo:     "comando",
		De:       "dashboard-cliente",
		Para:     idAlvo,
		Conteudo: bytesConteudo,
	}

	// Serializa e envia com \n para o bufio.Scanner do servidor ler
	dados, _ := json.Marshal(msg)
	_, err := fmt.Fprintf(c.conexao, "%s\n", dados)

	if err != nil {
		fmt.Println("⚠️ Conexão perdida. Tentando reconectar...")
		c.conectar()
	} else {
		fmt.Println("✅ Comando enviado.")
	}
}
