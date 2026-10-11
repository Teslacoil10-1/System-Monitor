package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"broker/internal/pb"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type MetricResponse struct {
	NodeID           string    `json:"node_id"`
	CPUUsage         float64   `json:"cpu_usage"`
	TransportLatency float64   `json:"transport_latency_ms"`
	Timestamp        time.Time `json:"timestamp"`
}

type Hub struct {
	clients   map[*websocket.Conn]bool
	broadcast chan MetricResponse
	mutex     sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan MetricResponse, 100),
	}
}

func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mutex.Lock()
		for client := range h.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("Websocket write error: %v", err)
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mutex.Unlock()
	}
}

var hub = NewHub()

func HandleWs(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade conn err: %v\n", err)
		return
	}

	hub.mutex.Lock()
	hub.clients[wsConn] = true
	hub.mutex.Unlock()
	log.Print("connected to websocket successfully\n")
	defer func() {
		hub.mutex.Lock()
		delete(hub.clients, wsConn)
		hub.mutex.Unlock()
		wsConn.Close()
	}()

	for {
		_, _, err := wsConn.ReadMessage()
		if err != nil {
			log.Printf("Client disconnected or err: %v", err)
			break
		}
	}
}

func streamMetricsFromDaemon() {
	serverAddr := "localhost:50051"

	for {
		conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("failed to connect to grpc error: %v, retrying...", err)
			time.Sleep(3 * time.Second)
			continue
		}

		client := pb.NewMetricServiceClient(conn)
		req := &pb.StreamRequest{
			NodeId:       "node1",
			SampleRateMs: 1000,
		}

		fmt.Printf("connected to grpc server daemon at: %v\n", serverAddr)

		stream, err := client.StreamMetrics(context.Background(), req)
		if err != nil {
			log.Printf("err opening stream: %v, retrying...", err)
			conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		for {
			snapshot, err := stream.Recv()
			if err == io.EOF {
				log.Println("daemon ended conn stream, attempting reconnect")
				break
			}
			if err != nil {
				log.Printf("err reading stream: %v, attempting reconnect", err)
				break
			}

			now := time.Now().UnixNano()
			latencyMs := float64(now-snapshot.GetTimestampNs()) / 1e6

			responsePayload := MetricResponse{
				NodeID:           snapshot.GetNodeId(),
				CPUUsage:         snapshot.GetCpu().GetUsage(),
				TransportLatency: latencyMs,
				Timestamp:        time.Unix(0, snapshot.GetTimestampNs()),
			}

			select {
			case hub.broadcast <- responsePayload:
			default:
				log.Println("broadcast channel full, dropping telemetry frame")
			}
		}

		conn.Close()
		time.Sleep(3 * time.Second)
	}
}

func main() {
	go hub.Run()
	go streamMetricsFromDaemon()

	http.HandleFunc("/ws", HandleWs)

	log.Println("starting ws broker server on :8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
