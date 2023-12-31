package main

import (
	pb "Replication/proto"
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	cursor "atomicgo.dev/cursor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var serverPorts []string
var serverConns []*grpc.ClientConn
var changeServerPortsLock sync.Mutex
var changeServerConnsLock sync.Mutex
var helpMenuCommand = "/h"
var resultCommand = "/r"
var currentIndexToRemove = 0

func joinChannel(ctx context.Context, client pb.AuctionServiceClient, port string) { //, Lamport int) {

	channel := pb.Channel{Name: *channelName, SendersName: *senderName}
	f := setLog(*senderName)
	defer f.Close()
	stream, err := client.JoinChannel(ctx, &channel)

	if err != nil {
		log.Printf("client.JoinChannel(ctx, &channel) throws: %v\n", err)
	}

	waitc := make(chan struct{})
	go func() {
		for {
			incoming, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				if status.Code(err) == codes.Unavailable {
					log.Printf("Connection lost with the server: %v", err)
					removeServerPort(port)
					removeServerConn()
					return
				}
				log.Fatalf("Failed to receive message from server. \nError: %v", err)
			}

			incrLamport(incoming)
		}
	}()

	<-waitc

}

func sendMessage(ctx context.Context, client pb.AuctionServiceClient, message string) { //, Lamport int) {
	if client == nil {
		log.Println("Client is nil, cannot send message")
		return
	}
	stream, err := client.SendMessage(ctx)

	if err != nil {
		log.Printf("Cannot send message - Error: %v", err)
		fmt.Printf("Cannot send message - Error: %v", err)
		return
	}

	Lamport++
	msg := formatMessage(message)
	stream.Send(&msg)

	ack, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Cannot receive message - Error: %v", err)
		fmt.Printf("Cannot receive message - Error: %v", err)
	}

	log.Println("Bid", ack)
	fmt.Println("Bid", ack)

	intMessage, err := strconv.Atoi(message)
	bidstring := fmt.Sprint(ack.Bid)
	if err == nil {
		if ack.Bid > int32(intMessage) {
			// If bid is rejected: Send the highest bid from ack to all servers (to keep stale data away shuu shuu (〃￣ω￣〃ゞ)
			sendMessageToAllServers(ctx, bidstring)
		}

	} else if err != nil && message != resultCommand {
		print := "Could not convert bid message to integer"
		log.Println(print)
		fmt.Println(print)
	}
}

func sendMessageToAllServers(ctx context.Context, message string) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(serverPorts))

	for _, conn := range serverConns {
		if conn != nil {
			sendMessage(ctx, pb.NewAuctionServiceClient(conn), message)
		}
	}
	waitGroup.Wait()
}

// Used after receiving a message
func incrLamport(msg *pb.Message) {
	if msg.GetTimestamp() > Lamport {
		Lamport = msg.GetTimestamp() + 1
	} else {
		Lamport++
	}
	msg.Timestamp = Lamport
}

// Function from atomicgo.dev/cursor to clear the previous line in the console
func clearPreviousConsoleLine() {
	cursor.ClearLinesUp(1)
	cursor.StartOfLine()
}

// Function to format message to be printed to the client
func formatClientMessage(incoming *pb.Message) string {
	return fmt.Sprintf("Lamport time: %v\n[%v]: %v\n\n", incoming.GetTimestamp(), incoming.GetSender(), incoming.GetMessage())
}

func findServerPorts() {
	if *serverPortsFlag != "" {
		serverPortsSplit := strings.Split(*serverPortsFlag, ",")

		for _, port := range serverPortsSplit {
			connectionString := fmt.Sprint(":", port)
			addServerPort(connectionString)
		}

	} else {
		print := fmt.Sprintf("No server ports provided")
		log.Fatalln(print)
		//fmt.Println(print)
	}
}

func connectToServers() {

	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	for i := 0; i < len(serverPorts); i++ {
		log.Println("Dialing server", serverPorts[i])
		conn, err := grpc.Dial(serverPorts[i], opts...)

		if err != nil {
			print := fmt.Sprintf("Fail to Dial : %v \n", err)
			log.Println(print)
		} else {
			serverConns = append(serverConns, conn)

			print := fmt.Sprint("Successfully connected to port ", serverPorts[i])
			log.Println(print)

			log.Println("Connecting to server...")
			client := pb.NewAuctionServiceClient(conn)
			go joinChannel(context.Background(), client, serverPorts[i])
		}
	}
}

func formatMessage(message string) pb.Message {
	return pb.Message{
		Channel: &pb.Channel{
			Name:        *channelName,
			SendersName: *senderName},
		Message:   message,
		Sender:    *senderName,
		Timestamp: Lamport,
	}
}

func addServerPort(portConnectionString string) {
	changeServerPortsLock.Lock()
	log.Printf("Adding %v to client's known servers\n", portConnectionString)
	defer changeServerPortsLock.Unlock()

	lengthServerPorts := len(serverPorts)
	serverPorts = append(serverPorts, portConnectionString)

	if len(serverPorts) == lengthServerPorts+1 {
		log.Printf("Successfully added %v to client's known servers\n", portConnectionString)
	} else {
		log.Printf("Failed to add %v to client's known servers\n", portConnectionString)
	}
}

func removeServerPort(portConnectionString string) {
	changeServerPortsLock.Lock()
	log.Printf("Removing port %v from client's known servers\n", portConnectionString)
	defer changeServerPortsLock.Unlock()

	for i, port := range serverPorts {
		if port == portConnectionString {
			// Remove the port from the slice
			currentIndexToRemove = i
			serverPorts = append(serverPorts[:i], serverPorts[i+1:]...)
			break
		}
	}

	for _, port := range serverPorts {
		if port == portConnectionString {
			log.Printf("Failed to remove %v from client's known servers\n", portConnectionString)
			return
		}
	}
	log.Printf("Successfully removed %v from client's known servers\n", portConnectionString)
}

func removeServerConn() {
	changeServerConnsLock.Lock()
	log.Printf("Removing a conn from client's known servers\n")
	defer changeServerConnsLock.Unlock()

	lengthServerConns := len(serverConns)
	if currentIndexToRemove < len(serverConns)-1 {
		serverConns = append(serverConns[:currentIndexToRemove], serverConns[currentIndexToRemove+1:]...)
	} else {
		serverConns = serverConns[:currentIndexToRemove]
	}

	if len(serverConns) == lengthServerConns-1 {
		log.Printf("Successfully removed a conn from client's known servers\n")
	} else {
		log.Printf("Failed to remove a conn from client's known servers\n")
	}
}

func isNumeric(msg string) bool {
	var number int
	_, err := fmt.Sscanf(msg, "%d", &number)
	return err == nil
}

func trimAndLowercaseMsg(message string) string {
	return strings.TrimSpace(strings.ToLower(message))
}

func foreverScanForInputAndSend() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := trimAndLowercaseMsg(scanner.Text())

		if !utf8.ValidString(message) {
			fmt.Printf("\n[Invalid characters.]\n[Please ensure your message is UTF-8 encoded.]\n\n")
			continue
		} else if message == helpMenuCommand {
			printHelpMessage()
			continue
		} else if message == resultCommand || isNumeric(message) {
			clearPreviousConsoleLine()
			fmt.Print("\n[", *senderName, "]: ", message)
			go sendMessageToAllServers(context.Background(), message)
			fmt.Print("\n")
		} else {
			fmt.Printf("\n[Invalid input: %v]\n[Please type %v to see the help menu.]\n\n", message, helpMenuCommand)
			continue
		}
	}
}

func printWelcome() {
	fmt.Println("\n ━━━━━⊱⊱ ⋆  AUCTION HOUSE ⋆ ⊰⊰━━━━━")
	fmt.Println("⋆｡˚ ☁︎ ˚｡ Welcome to " + *channelName)
	fmt.Println("⋆｡˚ ☁︎ ˚｡ Your username's " + *senderName)
	fmt.Println("⋆｡˚ ☁︎ ˚｡ To see the help menu, type " + helpMenuCommand)
	fmt.Printf("⋆｡˚ ☁︎ ˚｡ Enjoy the auction! \n\n")
}

func printHelpMessage() {
	fmt.Println("\n ━━━━━⊱⊱ ⋆  THE HELP MENU ⋆ ⊰⊰━━━━━")
	fmt.Println("⋆｡˚ ☁︎ ˚｡ To bid, type an integer and press enter")
	fmt.Println("⋆｡˚ ☁︎ ˚｡ - Example: 420 (press enter)")
	fmt.Println("⋆｡˚ ☁︎ ˚｡ To see the current bid or result of the auction, type " + resultCommand)
	fmt.Println("⋆｡˚ ☁︎ ˚｡ To see the help menu, type " + helpMenuCommand)
	fmt.Fprintln(os.Stdout, []any{"⋆｡˚ ☁︎ ˚｡ To exit, press Ctrl + C\n"}...)
}

var randomInt, err = rand.Int(rand.Reader, big.NewInt(1000))
var formattedSenderName = fmt.Sprintf("Anon %v", randomInt)
var channelName = flag.String("channel", "Eepy Auction", "Channel name for bidding")
var senderName = flag.String("username", formattedSenderName, "Sender's name")
var serverPortsFlag = flag.String("ports", "", "Comma seperated list of server ports")

var Lamport int32 = 0

func main() {
	// Actual initilization
	flag.Parse()
	fmt.Print("\n")
	findServerPorts()
	connectToServers()
	printWelcome()
	foreverScanForInputAndSend()
}

func setLog(name string) *os.File {
	if _, err := os.Open(fmt.Sprintf("%s.txt", name)); err == nil {
		if err := os.Truncate(fmt.Sprintf("%s.txt", name), 0); err != nil {
			log.Printf("Failed to truncate: %v", err)
		}
	}

	// This connects to the log file/changes the output of the log information to the log.txt file.
	f, err := os.OpenFile(fmt.Sprintf("%s.txt", name), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}
