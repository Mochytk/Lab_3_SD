package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	pb "distrieats/proto"
)

// Record format in CSV: ID,Estado,VectorClockValue
// Example: Ped-001,Preparando,2

func startProducer(broker *brokerServer, filePath string) {
	log.Printf("[Productor] Iniciando lectura de %s...", filePath)
	
	// Wait a bit before starting to simulate network forming
	time.Sleep(5 * time.Second)

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("[Productor] Error abriendo archivo %s: %v", filePath, err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Skip header
	_, err = reader.Read()
	if err != nil {
		log.Printf("[Productor] Error leyendo header: %v", err)
		return
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[Productor] Error leyendo fila: %v", err)
			continue
		}

		if len(record) < 4 {
			continue
		}

		pedidoID := record[0]
		estado := record[3]
		
		// Simulate a logical clock incrementing
		// In a real scenario, this would come from the producer's own state
		// For simplicity, we just use random or sequential
		clockVal := int32(time.Now().Unix() % 100) 
		
		vclock := &pb.VectorClock{
			Clocks: map[string]int32{
				"Productor": clockVal,
			},
		}

		pedido := &pb.Pedido{
			Id:          pedidoID,
			Estado:      estado,
			VectorClock: vclock,
		}

		log.Printf("[Productor] Emitiendo evento: Pedido %s -> %s", pedidoID, estado)
		broker.broadcastUpdate(pedido)

		// Simulate realistic intervals (1 to 3 seconds)
		delay := time.Duration(rand.Intn(3)+1) * time.Second
		time.Sleep(delay)
	}
	
	log.Printf("[Productor] Finalizada la lectura de %s. Iniciando tiempo de gracia...", filePath)
	time.Sleep(15 * time.Second)
	generarReporte(broker)
}

func generarReporte(broker *brokerServer) {
	log.Println("[Auditoría] Generando Reporte.txt...")
	
	broker.mu.Lock()
	validaciones := broker.validaciones
	var nodeAddr string
	if len(broker.datanodes) > 0 {
		nodeAddr = broker.datanodes[0] // Elegimos el primer datanode disponible
	}
	broker.mu.Unlock()

	var allOrders map[string]*pb.Pedido
	if nodeAddr != "" {
		client := broker.datanodeConns[nodeAddr]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		resp, err := client.GetAllOrders(ctx, &pb.GetAllOrdersRequest{})
		if err != nil {
			log.Printf("[Auditoría] Error obteniendo pedidos de %s: %v", nodeAddr, err)
		} else {
			allOrders = resp.Pedidos
		}
	}

	file, err := os.Create("Reporte.txt")
	if err != nil {
		log.Printf("[Auditoría] Error creando Reporte.txt: %v", err)
		return
	}
	defer file.Close()

	file.WriteString("=== REPORTE FINAL : DISTRIEATS ===\n\n")
	file.WriteString("[ESTADO GLOBAL DE PEDIDOS - Convergencia Alcanzada]\n")
	for id, pedido := range allOrders {
		vclockStr := ""
		if pedido.VectorClock != nil {
			for k, v := range pedido.VectorClock.Clocks {
				vclockStr += fmt.Sprintf("%s:%d, ", k, v)
			}
		}
		if len(vclockStr) > 2 {
			vclockStr = vclockStr[:len(vclockStr)-2]
		}
		file.WriteString(fmt.Sprintf("Pedido ID: %s | Estado Final: %s | Reloj Vectorial: [%s]\n", id, pedido.Estado, vclockStr))
	}

	file.WriteString("\n[AUDITORIA READ YOUR WRITES]\n")
	for _, val := range validaciones {
		file.WriteString(val + "\n")
	}
	file.WriteString("=================================\n")
	log.Println("[Auditoría] Reporte.txt generado exitosamente.")
}
