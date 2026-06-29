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

	// 0. Lectura de Estado Base
	log.Printf("[%s] Solicitando estado base del pedido %s...", *clientID, *pedidoID)
	ctx0, cancel0 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel0()
	
	baseResp, err := client.ConsultarEstado(ctx0, &pb.ConsultarEstadoRequest{
		ClienteId: *clientID,
		PedidoId:  *pedidoID,
	})
	
	if err != nil {
		log.Printf("[%s] Advertencia: Error consultando estado base: %v", *clientID, err)
	} else if baseResp.Encontrado {
		log.Printf("[%s] Estado base encontrado: %s", *clientID, baseResp.Pedido.Estado)
	} else {
		log.Printf("[%s] Estado base: Pedido no existe (Correcto).", *clientID)
	}

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
		
		// 3. Reportar Validación
		ctx3, cancel3 := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel3()
		_, _ = client.ReportarValidacion(ctx3, &pb.ReportarValidacionRequest{
			ClienteId:  *clientID,
			PedidoId:   *pedidoID,
			DatanodeId: readResp.DatanodeConsultado,
		})
	} else {
		log.Fatalf("[%s] ❌ ERROR RYW: Pedido %s NO ENCONTRADO en la lectura subsecuente. Falló la afinidad de sesión.", *clientID, *pedidoID)
	}
}
