package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/Glide28/go_hw_10/ledger"
	ledgerrpc "github.com/Glide28/go_hw_10/ledger/rpc"
	"google.golang.org/grpc"
)

func main() {
	service, closeFn, err := ledger.OpenService(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer closeFn()
	address := os.Getenv("LEDGER_LISTEN_ADDR")
	if address == "" {
		address = ":9090"
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	ledgerrpc.Register(server, service)
	log.Printf("Ledger gRPC server is listening on %s", address)
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
