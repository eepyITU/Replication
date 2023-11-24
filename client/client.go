package main

import (
	pb "Replication/proto"
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	cursor "atomicgo.dev/cursor"
	"github.com/inancgumus/screen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var serverPorts []string
var addServerPortsLock sync.Mutex

func joinChannel(ctx context.Context, client pb.AuctionServiceClient) { //, Lamport int) {

	channel := pb.Channel{Name: *channelName, SendersName: *senderName}
	f := setLog(*senderName)
	defer f.Close()
	stream, err := client.JoinChannel(ctx, &channel)

	if err != nil {
		log.Fatalf("client.JoinChannel(ctx, &channel) throws: %v", err)
	}

	// Send the join message to the server
	sendMessage(ctx, client, "9cbf281b855e41b4ad9f97707efdd29d")

	waitc := make(chan struct{})
	go func() {
		// The for loop is an infinite loop: Won't ever exit unless stream is closed
		for {
			// Receive message from stream and store it in incoming and possible error in err
			incoming, err := stream.Recv()

			// The stream closes when the client disconnects from the server, or vice versa
			// So if err == io.EOF, the stream is closed
			// If the stream is closed, close() is called on the waitc channel
			// Causing <-waitc to exit, thus ending the joinChannel func
			if err == io.EOF {
				close(waitc)
				return
			}

			if err != nil {
				log.Fatalf("Failed to receive message from channel joining. \nError: %v", err)
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

	// Increase Lamport timestamp before sending
	Lamport++

	// Create message
	msg := formatMessage(message)

	// Send message to server via stream
	stream.Send(&msg)

	// CloseAndRecv() closes the stream and returns the server's response, an ack
	ack, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Cannot send message - Error: %v", err)
		fmt.Printf("Cannot send message - Error: %v", err)
	}

	log.Printf("Message  %v \n", ack)
	fmt.Printf("Message  %v \n", ack)
}

func sendMessageToAllServers(ctx context.Context, message string) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(serverPorts))

	for _, port := range serverPorts {
		go pingServer(&waitGroup, port, message)
	}

	waitGroup.Wait()
}

// Function to increment the client's Lamport timestamp; used after receiving a message
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

func printWelcome() {
	fmt.Println("\n ━━━━━⊱⊱ ⋆  AUCTION HOUSE ⋆ ⊰⊰━━━━━")
	fmt.Println("⋆｡˚ ☁︎ ˚｡ Welcome to " + *channelName)
	fmt.Println("⋆｡˚ ☁︎ ˚｡ Your username's " + *senderName)
	fmt.Println("⋆｡˚ ☁︎ ˚｡ To exit, press Ctrl + C\n\n")
}

func findServerPorts() {
	var waitGroup sync.WaitGroup
	waitGroup.Add(80)
	start := 8000

	for i := 0; i < 81; i++ {
		go pingServer(&waitGroup, fmt.Sprintf(":%v", start+i), "")
	}
	waitGroup.Wait()
}

func pingServer(waitGroup *sync.WaitGroup, connectionString string, message string) {
	defer waitGroup.Done()

	log.Println("Pinging server: ", connectionString)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, connectionString, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return
	}

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Connection refused to close. Soz mate.")
		}
	}(conn)
	// No defer conn close???
	// - But isn't it line 162-163?
	// - So defer conn close is called when
	// - grpc.DialContext returns an error
	// - (AKA when the server is not found)
	// - Recall we're only calling pingServer to ports 8000-8080

	c := pb.NewAuctionServiceClient(conn)

	if err == nil {
		if slices.Contains(serverPorts, connectionString) {
			go sendMessage(ctx, c, message)
		} else {
			go joinChannel(ctx, c)
			addServerPort(connectionString)
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
	addServerPortsLock.Lock()
	log.Printf("Adding %v to client's known servers\n", portConnectionString)
	defer addServerPortsLock.Unlock()
	serverPorts = append(serverPorts, portConnectionString)
}

func foreverScanForInputAndSend() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if !utf8.ValidString(message) {
			fmt.Printf("\n[Invalid characters.]\n[Please ensure your message is UTF-8 encoded.]\n\n")
			continue
		}
		go sendMessageToAllServers(context.Background(), message)
	}
}

var channelName = flag.String("channel", "Eepy's Auction", "Channel name for bidding")
var senderName = flag.String("username", "Anon", "Sender's name")

// var tcpServerPort = flag.Int("server", 8000, "Tcp server")
var Lamport int32 = 0

func main() {
	// Cosmetics
	screen.Clear()
	screen.MoveTopLeft()
	time.Sleep(time.Second / 60)

	// Actual initilization
	flag.Parse()
	printWelcome()
	findServerPorts()
	foreverScanForInputAndSend()
}

// sets the logger to use a log.txt file instead of the console
func setLog(name string) *os.File {
	// Clears the log.txt file when a new server is started

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
