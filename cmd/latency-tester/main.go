package main

import (
	"context"
	"encoding/base64"
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

func main() {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		grpc_client()
	}()

	go func() {
		defer wg.Done()
		websocket_client()
	}()

	wg.Wait()
}

func grpc_client() {
	conn, err := grpc.Dial("sfo3.rpcpool.wg:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := pb.NewGeyserClient(conn)

	subr2 := pb.SubscribeRequest{}
	subr2.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	subr2.Accounts["usdc"] = &pb.SubscribeRequestFilterAccounts{Account: []string{"9wFFyRfZBsuAha4YcuxcXLKwMxJR43S7fPfQLusDBzvT"}}

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
	client, err := ws.Connect(context.Background(), "ws://sfo3.rpcpool.wg")
	if err != nil {
		panic(err)
	}
	program := solana.MustPublicKeyFromBase58("9wFFyRfZBsuAha4YcuxcXLKwMxJR43S7fPfQLusDBzvT") // serum

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
