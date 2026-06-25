package main

import (
	"context"
	"flag"
	"log"
	"time"

	pb "distrieats/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	gatewayAddr = flag.String("gateway", "localhost:50040", "Gateway address")
	clientID    = flag.String("client", "Cliente-1", "Client ID")
	pedidoID    = flag.String("pedido", "Ped-999", "Pedido ID")
)

func main() {
	flag.Parse()

	log.Printf("[%s] Iniciando cliente Hambriento, conectando a Gateway: %s", *clientID, *gatewayAddr)

	conn, err := grpc.Dial(*gatewayAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("[%s] No se pudo conectar al Gateway: %v", *clientID, err)
	}
	defer conn.Close()

	client := pb.NewClientServiceClient(conn)
	
	// Esperar un poco a que el sistema se inicialice en la simulación
	time.Sleep(10 * time.Second)

	// 1. Envío de Operación de Escritura
	log.Printf("[%s] Enviando CrearPedido (ID: %s)", *clientID, *pedidoID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createResp, err := client.CrearPedido(ctx, &pb.CrearPedidoRequest{
		ClienteId: *clientID,
		PedidoId:  *pedidoID,
	})
	
	if err != nil {
		log.Fatalf("[%s] Error creando pedido: %v", *clientID, err)
	}
	
	if !createResp.Exito {
		log.Fatalf("[%s] El Gateway rechazó la creación del pedido: %s", *clientID, createResp.Mensaje)
	}
	
	log.Printf("[%s] Pedido creado exitosamente. Datanode asignado por Broker: %s", *clientID, createResp.DatanodeAsignado)

	// 2. Lectura de Confirmación (Validación de Consistencia RYW)
	log.Printf("[%s] Consultando estado del pedido %s inmediatamente...", *clientID, *pedidoID)
	
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	
	readResp, err := client.ConsultarEstado(ctx2, &pb.ConsultarEstadoRequest{
		ClienteId: *clientID,
		PedidoId:  *pedidoID,
	})
	
	if err != nil {
		log.Fatalf("[%s] Error consultando pedido: %v", *clientID, err)
	}
	
	if readResp.Encontrado {
		log.Printf("[%s] ✅ VALIDACIÓN RYW EXITOSA: Pedido %s encontrado en Datanode %s. Estado: %s", 
			*clientID, readResp.Pedido.Id, readResp.DatanodeConsultado, readResp.Pedido.Estado)
	} else {
		log.Fatalf("[%s] ❌ ERROR RYW: Pedido %s NO ENCONTRADO en la lectura subsecuente. Falló la afinidad de sesión.", *clientID, *pedidoID)
	}
}
