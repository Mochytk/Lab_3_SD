package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	pb "distrieats/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	port          = flag.Int("port", 50050, "The server port")
	datanodesFlag = flag.String("datanodes", "", "Comma-separated list of datanode addresses (e.g. localhost:50051,localhost:50052,localhost:50053)")
	csvFile       = flag.String("csv", "pedidos.csv", "CSV file to read logistics events from")
)

type brokerServer struct {
	pb.UnimplementedClientServiceServer
	mu           sync.Mutex
	datanodes    []string
	rrIndex      int
	datanodeConns map[string]pb.DatanodeServiceClient
	validaciones []string // Almacena reportes de validaciones exitosas
}

func newBrokerServer(datanodeAddrs []string) *brokerServer {
	conns := make(map[string]pb.DatanodeServiceClient)
	for _, addr := range datanodeAddrs {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("[Broker] Error conectando a datanode %s: %v", addr, err)
			continue
		}
		conns[addr] = pb.NewDatanodeServiceClient(conn)
	}

	return &brokerServer{
		datanodes:    datanodeAddrs,
		datanodeConns: conns,
		validaciones: make([]string, 0),
	}
}

func (s *brokerServer) getNextDatanode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.datanodes) == 0 {
		return ""
	}
	node := s.datanodes[s.rrIndex]
	s.rrIndex = (s.rrIndex + 1) % len(s.datanodes)
	return node
}

func (s *brokerServer) CrearPedido(ctx context.Context, req *pb.CrearPedidoRequest) (*pb.CrearPedidoResponse, error) {
	for i := 0; i < len(s.datanodes); i++ {
		nodeAddr := s.getNextDatanode()
		if nodeAddr == "" {
			return &pb.CrearPedidoResponse{Exito: false, Mensaje: "No hay datanodes disponibles"}, nil
		}

		client := s.datanodeConns[nodeAddr]
		vclock := &pb.VectorClock{Clocks: map[string]int32{"Broker": 1}}

		pedido := &pb.Pedido{
			Id:          req.PedidoId,
			Estado:      "Recibido",
			VectorClock: vclock,
		}

		_, err := client.UpdateOrder(ctx, &pb.UpdateOrderRequest{Pedido: pedido})
		if err == nil {
			log.Printf("[Broker] Pedido %s creado y enrutado a %s", req.PedidoId, nodeAddr)
			return &pb.CrearPedidoResponse{Exito: true, Mensaje: "Pedido creado exitosamente", DatanodeAsignado: nodeAddr}, nil
		}
		log.Printf("[Broker] Error creando pedido en %s: %v. Reintentando...", nodeAddr, err)
	}
	return &pb.CrearPedidoResponse{Exito: false, Mensaje: "Error comunicando con datanodes, todos fallaron"}, nil
}

func (s *brokerServer) ConsultarEstado(ctx context.Context, req *pb.ConsultarEstadoRequest) (*pb.ConsultarEstadoResponse, error) {
	for i := 0; i < len(s.datanodes); i++ {
		nodeAddr := s.getNextDatanode()
		if nodeAddr == "" {
			return &pb.ConsultarEstadoResponse{Encontrado: false}, nil
		}

		client := s.datanodeConns[nodeAddr]
		resp, err := client.GetOrder(ctx, &pb.GetOrderRequest{PedidoId: req.PedidoId})
		if err == nil {
			log.Printf("[Broker] Consulta de pedido %s enrutada a %s", req.PedidoId, nodeAddr)
			return &pb.ConsultarEstadoResponse{
				Encontrado:         resp.Encontrado,
				Pedido:             resp.Pedido,
				DatanodeConsultado: nodeAddr,
			}, nil
		}
		log.Printf("[Broker] Error consultando pedido en %s: %v. Reintentando...", nodeAddr, err)
	}
	return &pb.ConsultarEstadoResponse{Encontrado: false}, nil
}

func (s *brokerServer) ReportarValidacion(ctx context.Context, req *pb.ReportarValidacionRequest) (*pb.ReportarValidacionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := fmt.Sprintf("- Cliente %s (Pedido: %s): Validacion Exitosa en Datanode %s", req.ClienteId, req.PedidoId, req.DatanodeId)
	s.validaciones = append(s.validaciones, msg)
	log.Printf("[Broker] %s", msg)
	return &pb.ReportarValidacionResponse{Exito: true}, nil
}

func (s *brokerServer) broadcastUpdate(pedido *pb.Pedido) {
	for addr, client := range s.datanodeConns {
		go func(a string, c pb.DatanodeServiceClient) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := c.UpdateOrder(ctx, &pb.UpdateOrderRequest{Pedido: pedido})
			if err != nil {
				log.Printf("[Broker] Error enviando broadcast a %s: %v", a, err)
			}
		}(addr, client)
	}
}

func main() {
	flag.Parse()

	var datanodes []string
	if *datanodesFlag != "" {
		datanodes = strings.Split(*datanodesFlag, ",")
	} else {
		log.Fatal("Debe especificar al menos un datanode (--datanodes)")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	broker := newBrokerServer(datanodes)
	
	// Start producer simulation in background
	go startProducer(broker, *csvFile)

	s := grpc.NewServer()
	pb.RegisterClientServiceServer(s, broker)

	log.Printf("Broker Central escuchando en puerto %d...", *port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
