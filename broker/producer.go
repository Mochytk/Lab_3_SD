package main

import (
	"encoding/csv"
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

		if len(record) < 2 {
			continue
		}

		pedidoID := record[0]
		estado := record[1]
		
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
	
	log.Printf("[Productor] Finalizada la lectura de %s", filePath)
}
