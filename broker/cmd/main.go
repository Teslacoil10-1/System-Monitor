package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"broker/internal/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	serverAddr := "localhost:50051"

	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to grpc error: %v", err)
	}

	defer conn.Close()

	client := pb.NewMetricServiceClient(conn)

	req := &pb.StreamRequest{
		NodeId:       "node1",
		SampleRateMs: 1000,
	}

	fmt.Printf("connected to grpc server daemon at: %v", serverAddr)

	stream, err := client.StreamMetrics(context.Background(), req)
	if err != nil {
		log.Fatalf("err opening stream: %v", err)
	}

	for {
		snapshot, err := stream.Recv()
		if err == io.EOF {
			log.Println("daemon ended conn stream")
			break
		}
		if err != nil {
			log.Fatalf("err reading stream: %v", err)
		}
		now := time.Now().UnixNano()
		latencyMs := float64(now-snapshot.TimestampNs) / 1e6

		log.Printf("[Node: %s] CPU Usage: %.2f%% | Transport Latency: %.3f ms",
			snapshot.NodeId,
			snapshot.Cpu.Usage,
			latencyMs,
		)
	}
}
