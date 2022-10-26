package main

import (
	"context"
	"encoding/base64"
	"flag"
	"io"
	"log"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"

	pb "github.com/rpcpool/latency-tester/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	wsAddr   = flag.String("ws", "", "Solana Websocket Address")
	grpcAddr = flag.String("grpc", "", "Solana gRPC address")
	account  = flag.String("account", "", "Account to subscribe to")
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		if *grpcAddr != "" {
			grpc_client()
		}
	}()

	go func() {
		defer wg.Done()

		if *wsAddr != "" {
			websocket_client()
		}
	}()

	wg.Wait()
}

func grpc_client() {
	conn, err := grpc.Dial(*grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := pb.NewGeyserClient(conn)

	subr2 := pb.SubscribeRequest{}
	subr2.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	subr2.Accounts["usdc"] = &pb.SubscribeRequestFilterAccounts{Account: []string{*account}}

	stream, err := client.Subscribe(context.Background(), &subr2)
	if err != nil {
		panic(err)
	}
	for {
		resp, err := stream.Recv()

		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}

		account := resp.GetAccount()

		log.Printf("[GRPC] %d: %s", account.Slot, base64.StdEncoding.EncodeToString(account.Account.Data))
	}
}

func websocket_client() {
	client, err := ws.Connect(context.Background(), *wsAddr)
	if err != nil {
		panic(err)
	}
	program := solana.MustPublicKeyFromBase58(*account) 

	sub, err := client.AccountSubscribeWithOpts(
		program,
		rpc.CommitmentProcessed,
		// You can specify the data encoding of the returned accounts:
		solana.EncodingBase64,
	)
	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()

	for {
		got, err := sub.Recv()
		if err != nil {
			panic(err)
		}

		log.Printf("[WebS] %d: %s", got.Context.Slot, base64.StdEncoding.EncodeToString(got.Value.Data.GetBinary()))
	}
}
