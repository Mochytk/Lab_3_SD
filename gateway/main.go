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
	port          = flag.Int("port", 50040, "The gateway port")
	brokerAddr    = flag.String("broker", "localhost:50050", "Broker address")
	datanodesFlag = flag.String("datanodes", "", "Comma-separated list of datanode addresses")
)

type stickySession struct {
	datanodeID string
	lastAccess time.Time
}

type gatewayServer struct {
	pb.UnimplementedClientServiceServer
	mu            sync.RWMutex
	sessions      map[string]stickySession // client_id -> session info
	sessionTTL    time.Duration
	brokerClient  pb.ClientServiceClient
	datanodeConns map[string]pb.DatanodeServiceClient // Address -> Client
	
	// Para propósitos de demostración, mapeamos datanode "id" a su dirección.
	// En un diseño real, el Broker debería devolver la IP/puerto del datanode, no solo el ID,
	// o el Gateway debería tener un mecanismo de descubrimiento.
	// Por ahora asumiremos que la lista datanodesFlag está en orden y
	// los IDs son DN1, DN2, DN3, correspondiendo al index.
	datanodeAddrs []string
}

func newGatewayServer(broker string, datanodes []string) *gatewayServer {
	// Connect to broker
	brokerConn, err := grpc.Dial(broker, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("[Gateway] No se pudo conectar al Broker: %v", err)
	}

	// Connect to datanodes
	conns := make(map[string]pb.DatanodeServiceClient)
	for _, addr := range datanodes {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("[Gateway] No se pudo conectar a Datanode %s: %v", addr, err)
			continue
		}
		conns[addr] = pb.NewDatanodeServiceClient(conn)
	}

	server := &gatewayServer{
		sessions:      make(map[string]stickySession),
		sessionTTL:    5 * time.Minute,
		brokerClient:  pb.NewClientServiceClient(brokerConn),
		datanodeConns: conns,
		datanodeAddrs: datanodes,
	}

	go server.cleanExpiredSessions()
	return server
}

func (s *gatewayServer) cleanExpiredSessions() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for clientID, session := range s.sessions {
			if now.Sub(session.lastAccess) > s.sessionTTL {
				delete(s.sessions, clientID)
				log.Printf("[Gateway] Sesión expirada para cliente %s", clientID)
			}
		}
		s.mu.Unlock()
	}
}

func (s *gatewayServer) getAffinity(clientID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if session, exists := s.sessions[clientID]; exists {
		// Update last access conceptually
		return session.datanodeID
	}
	return ""
}

func (s *gatewayServer) setAffinity(clientID, datanodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[clientID] = stickySession{
		datanodeID: datanodeID,
		lastAccess: time.Now(),
	}
}

func (s *gatewayServer) CrearPedido(ctx context.Context, req *pb.CrearPedidoRequest) (*pb.CrearPedidoResponse, error) {
	log.Printf("[Gateway] Recibida solicitud CrearPedido de cliente %s (Pedido: %s)", req.ClienteId, req.PedidoId)
	
	// Forward to broker for load balancing and processing
	resp, err := s.brokerClient.CrearPedido(ctx, req)
	if err != nil {
		log.Printf("[Gateway] Error reenviando CrearPedido al Broker: %v", err)
		return nil, err
	}

	if resp.Exito {
		s.setAffinity(req.ClienteId, resp.DatanodeAsignado)
		log.Printf("[Gateway] Afinidad guardada: Cliente %s -> Datanode %s", req.ClienteId, resp.DatanodeAsignado)
	}

	return resp, nil
}

func (s *gatewayServer) ConsultarEstado(ctx context.Context, req *pb.ConsultarEstadoRequest) (*pb.ConsultarEstadoResponse, error) {
	log.Printf("[Gateway] Recibida solicitud ConsultarEstado de cliente %s (Pedido: %s)", req.ClienteId, req.PedidoId)
	
	targetAddr := s.getAffinity(req.ClienteId)
	
	if targetAddr != "" {
		// Enrutar directo al Datanode (RYW)
		log.Printf("[Gateway] Afinidad encontrada para %s, enrutando a %s", req.ClienteId, targetAddr)
		client, exists := s.datanodeConns[targetAddr]
		if exists {
			resp, err := client.GetOrder(ctx, &pb.GetOrderRequest{PedidoId: req.PedidoId})
			if err != nil {
				log.Printf("[Gateway] Error consultando al Datanode %s: %v", targetAddr, err)
				return nil, err
			}
			return &pb.ConsultarEstadoResponse{
				Encontrado:         resp.Encontrado,
				Pedido:             resp.Pedido,
				DatanodeConsultado: targetAddr,
			}, nil
		}
	}
	
	// Sin afinidad o Datanode no encontrado, enrutar al broker
	log.Printf("[Gateway] Sin afinidad para %s, enrutando al Broker", req.ClienteId)
	return s.brokerClient.ConsultarEstado(ctx, req)
}

func main() {
	flag.Parse()

	var datanodes []string
	if *datanodesFlag != "" {
		datanodes = strings.Split(*datanodesFlag, ",")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	gateway := newGatewayServer(*brokerAddr, datanodes)

	s := grpc.NewServer()
	pb.RegisterClientServiceServer(s, gateway)

	log.Printf("Gateway de Pedidos escuchando en puerto %d...", *port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
