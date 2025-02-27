package main

import (
	"context"
	"flag"
	"fmt"

	"fenrir/internal/protocol"
	server "fenrir/internal/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultType    = "server"
	defaultAddress = "127.0.0.1"
	defaultPort    = 8081
)

func runDebugClient(serverAddress string, serverPort uint16) error {
	// Set up a GRPC Client connection.
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient(fmt.Sprintf("%s:%d", serverAddress, serverPort), opts...)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Pass the connection to our connection object and query server information.
	client := protocol.NewDebugClient(conn)
	info, err := client.QueryServer(context.TODO(), &protocol.Empty{})
	if err != nil {
		return err
	}
	println("Type:        ", info.Type.String())
	println("Id:          ", info.GetId())
	println("Port:        ", info.GetPort())
	println("Connections: ", info.GetConnections())
	return nil
}

func runDebugServer(address string, port uint16) error {
	ctx, cancel := context.WithCancel(context.Background())
	srv := server.NewServer(ctx, cancel, 0, address, port)

	// Startup the server.
	go srv.Run()

	// Wait until the context is finished.
	<-ctx.Done()
	return nil
}

func main() {
	// Command line parameters.
	flagType := flag.String("type", defaultType, "Type of executable to run - [server | client]")
	flagAddress := flag.String("address", defaultAddress, "Server address to connect to")
	flagPort := flag.Uint("port", defaultPort, "Server port to connect to")
	flag.Parse()

	switch *flagType {
	case "server":
		err := runDebugServer(*flagAddress, uint16(*flagPort))
		if err != nil {
			println("Error, ", err.Error())
		}
	case "client":
		err := runDebugClient(*flagAddress, uint16(*flagPort))
		if err != nil {
			println("Error, ", err.Error())
		}

	default:
		println("Please input a valid type (server|client)")
	}
}
