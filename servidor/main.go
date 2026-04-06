package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// --- ESTRUTURAS DE DADOS ---

type Dispositivo struct {
	ID                string    `json:"id"`
	Nome              string    `json:"nome"`
	Tipo              string    `json:"tipo"` // "sensor" ou "atuador"
	Temperatura       float64   `json:"temperatura"`
	TemperaturaAlvo   float64   `json:"temperatura_alvo"`
	Status            string    `json:"status"` // "ligado"/"desligado" ou "ativo"
	UltimaAtualizacao time.Time `json:"ultima_atualizacao"`
	Endereco          string    `json:"endereco"`          // IP real detectado via UDP/TCP
	SensorPareadoID   string    `json:"sensor_pareado_id"` // ID do sensor que este atuador controla
	ConexaoTCP        net.Conn  `json:"-"`                 // Ignorado no JSON para evitar recursão
}

type Mensagem struct {
	Tipo     string          `json:"tipo"` // "dados_sensor", "comando", "registrar", "ajustar_simulacao"
	De       string          `json:"de"`
	Para     string          `json:"para"`
	Conteudo json.RawMessage `json:"conteudo"`
}

type ServicoIntegracao struct {
	dispositivos map[string]*Dispositivo
	mutex        sync.RWMutex
	clientesTCP  map[net.Conn]bool
}

var servico = &ServicoIntegracao{
	dispositivos: make(map[string]*Dispositivo),
	clientesTCP:  make(map[net.Conn]bool),
}

func main() {
	fmt.Println("========================================")
	fmt.Println("🚀 SISTEMA INDUSTRIAL DISTRIBUÍDO v3.0")
	fmt.Println("========================================")

	// Inicia os serviços em Goroutines
	go iniciarServidorTCP()  // Porta 8080: Atuadores e Comandos
	go iniciarServidorUDP()  // Porta 8082: Dados de Sensores
	go iniciarServidorHTTP() // Porta 8081: Dashboard/API
	go monitorarSistema()    // Logs periódicos

	select {} // Bloqueia a main para manter o servidor rodando
}

// --- SERVIDOR TCP (COMANDOS E ATUADORES) ---
func iniciarServidorTCP() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Printf("❌ Falha TCP: %v\n", err)
		return
	}
	fmt.Println("✅ TCP: Escutando na porta 8080 (Atuadores)")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go tratarConexaoTCP(conn)
	}
}

func tratarConexaoTCP(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()  //Pega endereço e converte para String
	ip, _, _ := net.SplitHostPort(remoteAddr) //Separa IP de Porta

	servico.mutex.Lock()
	servico.clientesTCP[conn] = true
	servico.mutex.Unlock()

	defer func() {
		servico.mutex.Lock()
		delete(servico.clientesTCP, conn)
		// percorre os dispositivos (atuadores)
		for _, d := range servico.dispositivos {
			if d.ConexaoTCP == conn {
				d.ConexaoTCP = nil
				fmt.Printf("🔌 [TCP] Dispositivo %s desconectado\n", d.ID)
			}
		}
		servico.mutex.Unlock()
		conn.Close()
	}()

	dadosRecebidos := bufio.NewScanner(conn)
	//Loop infinito até parar de receber dados

	for dadosRecebidos.Scan() {
		var msg Mensagem
		//Jason (bytes) -> struct Go (msg)
		if err := json.Unmarshal(dadosRecebidos.Bytes(), &msg); err != nil {
			continue
		}

		switch msg.Tipo {
		case "registrar":
			var dTemp Dispositivo
			if err := json.Unmarshal(msg.Conteudo, &dTemp); err != nil {
				continue
			}

			//Cadastro de dispositivo
			servico.mutex.Lock()
			if d, existe := servico.dispositivos[dTemp.ID]; existe {
				d.ConexaoTCP = conn
				d.Endereco = ip
				d.Status = dTemp.Status
				d.UltimaAtualizacao = time.Now()
				if dTemp.SensorPareadoID != "" {
					d.SensorPareadoID = dTemp.SensorPareadoID
				}
			} else {

				dTemp.ConexaoTCP = conn
				dTemp.Endereco = ip
				dTemp.UltimaAtualizacao = time.Now()
				servico.dispositivos[dTemp.ID] = &dTemp
				//Talvez aqui tenha d.SensorPareadoID = dTemp.SensorPareadoID
			}
			servico.mutex.Unlock()
			fmt.Printf("✅ [TCP] Atuador registrado: %s em %s\n", dTemp.ID, ip)

		case "comando":
			servico.mutex.RLock()
			atuador, existe := servico.dispositivos[msg.Para]
			servico.mutex.RUnlock()

			if existe && atuador.ConexaoTCP != nil {
				// 1. Repassa o comando original para o atuador
				fmt.Fprintf(atuador.ConexaoTCP, "%s\n", dadosRecebidos.Text())

				var dadosCmd map[string]interface{}
				json.Unmarshal(msg.Conteudo, &dadosCmd)

				// 2. Atualiza o status do atuador no servidor
				servico.mutex.Lock()
				if status, ok := dadosCmd["status"].(string); ok {
					atuador.Status = status
					fmt.Printf("📝 [TCP] Atuador %s atualizado: status=%s\n", atuador.ID, status)
				}

				// Se tiver target_temperature, atualiza também
				if targetTemp, ok := dadosCmd["target_temperature"].(float64); ok {
					atuador.TemperaturaAlvo = targetTemp
					fmt.Printf("📝 [TCP] Atuador %s atualizado: Alvo=%.1f°C\n", atuador.ID, targetTemp)
				}
				atuador.UltimaAtualizacao = time.Now()
				servico.mutex.Unlock()

				// Quando desligar, envia 0 para o sensor iniciar resfriamento passivo
				if status, ok := dadosCmd["status"].(string); ok && status == "desligado" {
					if atuador.SensorPareadoID != "" {
						// O sensor então usará sua lógica de resfriamento passivo
						go notificarSensorUDP(atuador.SensorPareadoID, 0.0)
						fmt.Printf("📉 [Sincronia] Sensor %s: resfriamento natural iniciado (alvo=0)\n", atuador.SensorPareadoID)
					}
				} else if val, ok := dadosCmd["target_temperature"].(float64); ok {
					if atuador.SensorPareadoID != "" {
						go notificarSensorUDP(atuador.SensorPareadoID, val)
					}
				}
			}
		}
	}
}

// --- SERVIDOR UDP (DADOS DE SENSORES) ---

func iniciarServidorUDP() {
	addr, _ := net.ResolveUDPAddr("udp", ":8082")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("❌ Falha UDP: %v\n", err)
		return
	}
	fmt.Println("✅ UDP: Escutando na porta 8082 (Sensores)")

	buffer := make([]byte, 4096)
	for {
		n, addrCli, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		// Cópia segura para a goroutine
		dados := make([]byte, n)
		copy(dados, buffer[:n])
		go tratarDadosUDP(dados, addrCli)
	}
}

func tratarDadosUDP(dados []byte, addr *net.UDPAddr) {
	var msg Mensagem
	if err := json.Unmarshal(dados, &msg); err != nil {
		return
	}

	ip, _, _ := net.SplitHostPort(addr.String())

	if msg.Tipo == "dados_sensor" {
		var dSens Dispositivo
		if err := json.Unmarshal(msg.Conteudo, &dSens); err != nil {
			return
		}

		servico.mutex.Lock()
		if d, existe := servico.dispositivos[dSens.ID]; existe {
			d.Temperatura = dSens.Temperatura
			d.UltimaAtualizacao = time.Now()
			d.Endereco = ip
		} else {
			dSens.Tipo = "sensor"
			dSens.Endereco = ip
			dSens.UltimaAtualizacao = time.Now()
			servico.dispositivos[dSens.ID] = &dSens
			fmt.Printf("✅ [UDP] Novo Sensor: %s em %s\n", dSens.ID, ip)
		}
		servico.mutex.Unlock()
	}
}

// --- UTILITÁRIOS E NOTIFICAÇÃO ---

func notificarSensorUDP(sensorID string, novaTemp float64) {
	servico.mutex.RLock()
	sensor, existe := servico.dispositivos[sensorID]
	ip := ""
	if existe {
		ip = sensor.Endereco
	}
	servico.mutex.RUnlock()

	if ip == "" {
		fmt.Printf("⚠️ [UDP] Sensor %s não encontrado para notificação\n", sensorID)
		return
	}

	// Porta 8083 é onde o binário do SENSOR deve estar escutando comandos
	addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:8083", ip))
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("⚠️ [UDP] Erro ao conectar ao sensor %s: %v\n", sensorID, err)
		return
	}
	defer conn.Close()

	// Payload diferente baseado no valor
	var payload []byte
	if novaTemp == 0.0 {
		// Quando é 0, significa "desligar aquecimento" (resfriamento natural)
		payload, _ = json.Marshal(map[string]interface{}{
			"temperatura_alvo": 0.0,
			"comando":          "resfriar_natural",
		})
	} else {
		payload, _ = json.Marshal(map[string]interface{}{
			"temperatura_alvo": novaTemp,
		})
	}

	msg := Mensagem{Tipo: "ajustar_simulacao", Conteudo: payload}
	b, _ := json.Marshal(msg)

	if _, err := conn.Write(b); err != nil {
		fmt.Printf("⚠️ [UDP] Erro ao enviar para sensor %s: %v\n", sensorID, err)
	} else {
		if novaTemp == 0.0 {
			fmt.Printf("📨 [UDP] Sensor %s notificado para RESFRIAMENTO NATURAL\n", sensorID)
		} else {
			fmt.Printf("📨 [UDP] Sensor %s notificado para ALVO %.1f°C\n", sensorID, novaTemp)
		}
	}
}

// --- API HTTP (DASHBOARD) ---

func iniciarServidorHTTP() {
	server, _ := net.Listen("tcp", ":8081")
	fmt.Println("✅ HTTP: API na porta 8081 (/devices)")

	for {
		conn, err := server.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			// Resposta simplificada para o browser/dashboard
			servico.mutex.RLock()
			dados, _ := json.Marshal(servico.dispositivos)
			servico.mutex.RUnlock()

			res := "HTTP/1.1 200 OK\r\n" +
				"Content-Type: application/json\r\n" +
				"Access-Control-Allow-Origin: *\r\n" +
				fmt.Sprintf("Content-Length: %d\r\n\r\n", len(dados)) +
				string(dados)
			c.Write([]byte(res))
		}(conn)
	}
}

func monitorarSistema() {
	for range time.Tick(15 * time.Second) {
		servico.mutex.RLock()
		fmt.Printf("\n--- 📈 STATUS ATUAL: %d DISPOSITIVOS | %d CONEXÕES TCP ---\n",
			len(servico.dispositivos), len(servico.clientesTCP))

		// Mostra detalhes dos atuadores
		for id, d := range servico.dispositivos {
			if d.Tipo == "atuador" {
				fmt.Printf("  🔧 Atuador %s: Status=%s, Alvo=%.1f°C, SensorPareado=%s\n",
					id, d.Status, d.TemperaturaAlvo, d.SensorPareadoID)
			} else if d.Tipo == "sensor" {
				fmt.Printf("  🌡️ Sensor %s: Temp=%.2f°C, Status=%s\n",
					id, d.Temperatura, d.Status)
			}
		}
		servico.mutex.RUnlock()
	}
}
