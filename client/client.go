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
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	cursor "atomicgo.dev/cursor"
	"github.com/inancgumus/screen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var serverPorts []string
var serverConns []*grpc.ClientConn
var changeServerPortsLock sync.Mutex
var helpMenuCommand = "/h"
var resultCommand = "/r"

func joinChannel(ctx context.Context, client pb.AuctionServiceClient) { //, Lamport int) {

	channel := pb.Channel{Name: *channelName, SendersName: *senderName}
	f := setLog(*senderName)
	defer f.Close()
	stream, err := client.JoinChannel(ctx, &channel)

	if err != nil {
		log.Fatalf("client.JoinChannel(ctx, &channel) throws: %v", err)
	}

	//sendMessage(ctx, client, "9cbf281b855e41b4ad9f97707efdd29d")

	waitc := make(chan struct{})
	go func() {
		for {
			incoming, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}

			if err != nil {
				log.Fatalf("Failed to receive message from client joining. \nError: %v", err)
			}

			incrLamport(incoming)

			messageFormat := "Received at " + formatClientMessage(incoming)

			if *senderName == incoming.GetSender() {
				if incoming.GetMessage() != fmt.Sprintf("Participant %v joined Auction at Lamport time %v", incoming.GetSender(), incoming.GetTimestamp()-3) {
					clearPreviousConsoleLine()
				}
				log.Print(messageFormat)
				fmt.Print(messageFormat)
			} else {
				log.Print(messageFormat)
				fmt.Print(messageFormat)
			}
		}
	}()

	<-waitc

}

func sendMessage(ctx context.Context, client pb.AuctionServiceClient, message string) { //, Lamport int) {
	stream, err := client.SendMessage(ctx)
	if err != nil {
		log.Printf("Cannot send message - Error: %v", err)
		fmt.Printf("Cannot send message - Error: %v", err)
	}

	Lamport++
	msg := formatMessage(message)
	stream.Send(&msg)

	ack, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Cannot send message - Error: %v", err)
		fmt.Printf("Cannot send message - Error: %v", err)
	}

	log.Println("Message ", ack)
	fmt.Println("Message ", ack)
}

func sendMessageToAllServers(ctx context.Context, message string) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(serverPorts))

	for _, conn := range serverConns {
		sendMessage(ctx, pb.NewAuctionServiceClient(conn), message)
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
		serverPorts = strings.Split(*serverPortsFlag, ",")

		for _, port := range serverPorts {
			addServerPort(port)
		}

	} else {
		print := fmt.Sprintf("No server ports provided")
		log.Fatalln(print)
		fmt.Println(print)
	}
}

func connectToServers() {

	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	for _, port := range serverPorts {

		for i := 0; i < 3; i++ {
			i, err := grpc.Dial(serverPorts[i], opts...)

			if err != nil {
				print := fmt.Sprintf("Fail to Dial : %v \n", err)
				fmt.Printf(print)
				log.Printf(print)
				return
			}

			serverConns = append(serverConns, i)

			print := fmt.Sprintf("Successfully connected to port ", port)
			log.Println(print)
			fmt.Println(print)
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
	log.Printf("Removing %v from client's known servers\n", portConnectionString)
	defer changeServerPortsLock.Unlock()

	lengthServerPorts := len(serverPorts)
	serverPorts = append(serverPorts[:lengthServerPorts], serverPorts[lengthServerPorts-1:]...)

	if len(serverPorts) == lengthServerPorts-1 {
		log.Printf("Successfully removed %v from client's known servers\n", portConnectionString)
	} else {
		log.Printf("Failed to remove %v from client's known servers\n", portConnectionString)
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
		} else if message != resultCommand || message != helpMenuCommand || !isNumeric(message) {
			fmt.Printf("\n[Invalid input.]\n[Please type %v to see the help menu.]\n\n", helpMenuCommand)
			continue
		} else {
			go sendMessageToAllServers(context.Background(), message)
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
	fmt.Println("⋆｡˚ ☁︎ ˚｡ To exit, press Ctrl + C\n\n")
}

var randomInt, err = rand.Int(rand.Reader, big.NewInt(1000))
var formattedSenderName = fmt.Sprintf("Anon %v", randomInt)
var channelName = flag.String("channel", "Eepy Auction", "Channel name for bidding")
var senderName = flag.String("username", formattedSenderName, "Sender's name")
var serverPortsFlag = flag.String("serverports", "", "Comma seperated list of server ports")

var Lamport int32 = 0

func main() {
	// Cosmetics
	screen.Clear()
	screen.MoveTopLeft()
	time.Sleep(time.Second / 60)

	// Actual initilization
	flag.Parse()
	findServerPorts()
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

// Step 1: Clients finds server ports by pinging all possible ports
// Step 2: Clients Join channel on all available serverports
//
