package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"flag"
	"io"
	"log"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"

	pb "github.com/rpcpool/latency-tester/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	wsAddr      = flag.String("ws", "", "Solana Websocket Address")
	grpcAddr    = flag.String("grpc", "", "Solana gRPC address")
	account     = flag.String("account", "", "Account to subscribe to")
	accountData = flag.Bool("account-data", true, "Include data")

	ws_slot   map[uint64]uint = make(map[uint64]uint)
	grpc_slot map[uint64]uint = make(map[uint64]uint)
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		if *wsAddr != "" {
			websocket_client()
		}
	}()
	go func() {
		defer wg.Done()
		if *grpcAddr != "" {
			grpc_client()
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
		timestamp := time.Now().UnixNano()

		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}

		account := resp.GetAccount()

		grpc_slot[account.Slot]++

		if *accountData {
			log.Printf("[GRPC] %d{%d}: %s @ %d", account.Slot, grpc_slot[account.Slot], base64.StdEncoding.EncodeToString(account.Account.Data), timestamp)
		} else {
			log.Printf("[GRPC] %d{%d}: %x @ %d", account.Slot, grpc_slot[account.Slot], md5.Sum(account.Account.Data), timestamp)
		}
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
		timestamp := time.Now().UnixNano()
		if err != nil {
			panic(err)
		}

		ws_slot[got.Context.Slot]++

		if *accountData {
			log.Printf("[WS  ] %d{%d}: %s @ %d", got.Context.Slot, ws_slot[got.Context.Slot], base64.StdEncoding.EncodeToString(got.Value.Data.GetBinary()), timestamp)
		} else {
			log.Printf("[WS  ] %d{%d} %x @ %d", got.Context.Slot, ws_slot[got.Context.Slot], md5.Sum(got.Value.Data.GetBinary()), timestamp)
		}
	}
}
