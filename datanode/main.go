package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	pb "distrieats/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	port        = flag.Int("port", 50051, "The server port")
	id          = flag.String("id", "DN1", "Datanode ID")
	peersFlag   = flag.String("peers", "", "Comma-separated list of peer addresses (e.g. localhost:50052,localhost:50053)")
	gossipInt   = flag.Int("gossip-interval", 5, "Gossip interval in seconds")
)

// Map of priorities for deterministic resolution
var statePriority = map[string]int{
	"Recibido":   1,
	"Preparando": 2,
	"En Camino":  3,
	"Entregado":  4,
	"Cancelado":  5,
}

type datanodeServer struct {
	pb.UnimplementedDatanodeServiceServer
	mu      sync.RWMutex
	pedidos map[string]*pb.Pedido
	peers   []string
}

func newServer(peers []string) *datanodeServer {
	return &datanodeServer{
		pedidos: make(map[string]*pb.Pedido),
		peers:   peers,
	}
}

// CompareVectorClocks returns:
// 1 if v1 > v2 (v1 is strictly newer)
// -1 if v1 < v2 (v1 is strictly older)
// 0 if v1 == v2 (identical)
// 2 if concurrent
func compareVectorClocks(v1, v2 *pb.VectorClock) int {
	if v1 == nil && v2 == nil {
		return 0
	}
	if v1 == nil {
		return -1
	}
	if v2 == nil {
		return 1
	}

	isGreater := false
	isLess := false

	// Check all keys from v1
	for node, val1 := range v1.Clocks {
		val2 := v2.Clocks[node] // Defaults to 0 if missing
		if val1 > val2 {
			isGreater = true
		} else if val1 < val2 {
			isLess = true
		}
	}

	// Check keys in v2 that are not in v1
	for node, val2 := range v2.Clocks {
		if _, exists := v1.Clocks[node]; !exists && val2 > 0 {
			isLess = true
		}
	}

	if isGreater && !isLess {
		return 1
	} else if !isGreater && isLess {
		return -1
	} else if !isGreater && !isLess {
		return 0
	} else {
		return 2 // Concurrent
	}
}

func mergeVectorClocks(v1, v2 *pb.VectorClock) *pb.VectorClock {
	merged := &pb.VectorClock{Clocks: make(map[string]int32)}
	if v1 != nil {
		for k, v := range v1.Clocks {
			merged.Clocks[k] = v
		}
	}
	if v2 != nil {
		for k, v := range v2.Clocks {
			if existing, ok := merged.Clocks[k]; ok {
				if v > existing {
					merged.Clocks[k] = v
				}
			} else {
				merged.Clocks[k] = v
			}
		}
	}
	return merged
}

func (s *datanodeServer) processUpdate(incoming *pb.Pedido) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.pedidos[incoming.Id]
	if !exists {
		// New order
		s.pedidos[incoming.Id] = incoming
		log.Printf("[Update] Pedido nuevo: %s -> %s", incoming.Id, incoming.Estado)
		return
	}

	// Compare vector clocks
	comp := compareVectorClocks(incoming.VectorClock, existing.VectorClock)

	mergedClock := mergeVectorClocks(existing.VectorClock, incoming.VectorClock)

	if comp == 1 {
		// Incoming is newer
		s.pedidos[incoming.Id] = &pb.Pedido{
			Id:          incoming.Id,
			Estado:      incoming.Estado,
			VectorClock: mergedClock,
		}
		log.Printf("[Update] Pedido actualizado: %s -> %s (reloj mayor)", incoming.Id, incoming.Estado)
	} else if comp == -1 {
		// Incoming is older, ignore state but merge clock
		existing.VectorClock = mergedClock
		log.Printf("[Update] Ignorando pedido %s -> %s (reloj menor), actualizando reloj", incoming.Id, incoming.Estado)
	} else if comp == 0 {
		// Identical, ignore
	} else { // comp == 2 (Concurrent)
		// Deterministic resolution
		priorityIncoming := statePriority[incoming.Estado]
		priorityExisting := statePriority[existing.Estado]

		if priorityIncoming > priorityExisting {
			s.pedidos[incoming.Id] = &pb.Pedido{
				Id:          incoming.Id,
				Estado:      incoming.Estado,
				VectorClock: mergedClock,
			}
			log.Printf("[Resolucion] Conflicto resuelto a favor del entrante: %s -> %s", incoming.Id, incoming.Estado)
		} else {
			existing.VectorClock = mergedClock
			log.Printf("[Resolucion] Conflicto resuelto a favor del local: %s -> %s (ignorado %s)", existing.Id, existing.Estado, incoming.Estado)
		}
	}
}

func (s *datanodeServer) UpdateOrder(ctx context.Context, req *pb.UpdateOrderRequest) (*pb.UpdateOrderResponse, error) {
	s.processUpdate(req.Pedido)
	return &pb.UpdateOrderResponse{Exito: true, Mensaje: "Actualizado"}, nil
}

func (s *datanodeServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pedido, exists := s.pedidos[req.PedidoId]
	if exists {
		return &pb.GetOrderResponse{Encontrado: true, Pedido: pedido}, nil
	}
	return &pb.GetOrderResponse{Encontrado: false}, nil
}

func (s *datanodeServer) GossipSync(ctx context.Context, req *pb.GossipRequest) (*pb.GossipResponse, error) {
	log.Printf("[Gossip] Recibido sync de %s", req.DatanodeId)
	
	// Apply incoming orders to our state
	for _, p := range req.Pedidos {
		s.processUpdate(p)
	}
	
	return &pb.GossipResponse{Exito: true}, nil
}

func (s *datanodeServer) startGossip() {
	if len(s.peers) == 0 {
		log.Println("[Gossip] No hay peers configurados para gossip.")
		return
	}

	ticker := time.NewTicker(time.Duration(*gossipInt) * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		peerIndex := rand.Intn(len(s.peers))
		peer := s.peers[peerIndex]

		go s.sendGossip(peer)
	}
}

func (s *datanodeServer) sendGossip(peerAddr string) {
	log.Printf("[Gossip] Iniciando sync con %s", peerAddr)

	conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("[Gossip] Error conectando a peer %s: %v", peerAddr, err)
		return
	}
	defer conn.Close()

	client := pb.NewDatanodeServiceClient(conn)
	
	s.mu.RLock()
	pedidosCopy := make(map[string]*pb.Pedido)
	for k, v := range s.pedidos {
		pedidosCopy[k] = v
	}
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.GossipSync(ctx, &pb.GossipRequest{
		DatanodeId: *id,
		Pedidos:    pedidosCopy,
	})

	if err != nil {
		log.Printf("[Gossip] Error enviando sync a %s: %v", peerAddr, err)
	} else {
		log.Printf("[Gossip] Sync completado con %s", peerAddr)
	}
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	var peers []string
	if *peersFlag != "" {
		peers = strings.Split(*peersFlag, ",")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	
	s := grpc.NewServer()
	datanode := newServer(peers)
	pb.RegisterDatanodeServiceServer(s, datanode)
	
	go datanode.startGossip()

	log.Printf("Datanode %s escuchando en puerto %d...", *id, *port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
