package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Glide28/go_hw_10/gateway/internal/api"
	"github.com/Glide28/go_hw_10/ledger"
	ledgerrpc "github.com/Glide28/go_hw_10/ledger/rpc"
)

func main() {
	ctx := context.Background()
	var application ledger.Service
	var closeFn func()
	var err error
	if address := os.Getenv("LEDGER_GRPC_ADDR"); address != "" {
		client, dialErr := ledgerrpc.Dial(address)
		if dialErr != nil {
			log.Fatal(dialErr)
		}
		application = client
		closeFn = func() { _ = client.Close() }
		log.Printf("Gateway uses Ledger gRPC service at %s", address)
	} else {
		application, closeFn, err = ledger.OpenService(ctx)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Gateway uses embedded Ledger service")
	}
	defer closeFn()

	handler := api.NewHandler(application)
	log.Println("Gateway is listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
