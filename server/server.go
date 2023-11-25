package main

import (
	pb "Replication/proto"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"strconv"

	"google.golang.org/grpc"
)

type AuctionServiceServer struct {
	pb.UnimplementedAuctionServiceServer
	channel           map[string][]chan *pb.Message
	Lamport           int32 // Remote timestamp; keeps local time for newly joined users
	CurrentHighestBid int32
}

func (s *AuctionServiceServer) JoinChannel(ch *pb.Channel, msgStream pb.AuctionService_JoinChannelServer) error {

	// Create a channel for the client
	clientChannel := make(chan *pb.Message)

	// Add the client-channel to the map
	s.channel[ch.Name] = append(s.channel[ch.Name], clientChannel)

	// doing this never closes the stream
	for {
		select {
		// if the client closes the stream / disconnects, the channel is closed
		case <-msgStream.Context().Done():

			//leaveString := fmt.Sprintf("%v has left the channel", ch.GetSendersName())
			//leaveString := fmt.Sprintf("Participant %v has left the Auction at Lamport time %v", ch.GetSendersName(), s.Lamport)

			// Remove the clientChannel from the slice of channels for this channel
			s.removeChannel(ch, clientChannel)

			// Simulate that the client has sent a farewell message to the server
			//s.Lamport++

			// Send a message to every client in the channel that a client has left
			//msg := &pb.Message{Sender: ch.Name, Message: leaveString, Channel: ch, Timestamp: s.Lamport}

			//s.sendMsgToClients(msg)

			// closes the function by returning nil.
			return nil

		// if a client sends a message, incr! :D Since server has RECEIVED a msg
		case msg := <-clientChannel:

			//s.incrLamport(msg)

			// stream sends the message to client
			msgStream.Send(msg)
		}
	}
}

// SendMessage function is called when a client sends a message.
// AKA this is where the serever receives a message from a client's stream that the other clients shall receive.
// When a client sends a message, we send the message to the channel.

func (s *AuctionServiceServer) SendMessage(msgStream pb.AuctionService_SendMessageServer) error {

	// Receive message from client
	msg, err := msgStream.Recv()

	s.incrLamport(msg)

	// if the stream is closed, return nil
	if err == io.EOF {
		return nil
	}

	// if there are errors, return the error
	if err != nil {
		return err
	}
	var ack pb.MessageAck
	// Check if message is a result, if its isnt it must be a bid.
	if msg.GetMessage() == "/r" {
		ack = pb.MessageAck{Status: string(s.CurrentHighestBid)}
	} else {
		// check if message is integer
		if s.validBid(msg.GetMessage()) {
			ack = pb.MessageAck{Status: "Bid Accepted"}
		} else {
			ack = pb.MessageAck{Status: "Bid Rejected"}
		}
	}
	// Acknowledge message received to client
	msgStream.SendAndClose(&ack)

	//s.sendMsgToClients(msg)

	return nil
}

// Check whether or not the given client message in an integer (and therefore a bid.)
func (s *AuctionServiceServer) validBid(msg string) bool {
	Bid, err := strconv.Atoi(msg)
	if err != nil {
		fmt.Println("Error: " + err.Error())
		return false
	}
	return Bid > int(s.CurrentHighestBid)
}

// Function to increase server's Lamport timestamp; used after receiving a message
func (s *AuctionServiceServer) incrLamport(msg *pb.Message) {
	if msg.GetTimestamp() > s.Lamport {
		s.Lamport = msg.GetTimestamp() + 1
	} else {
		s.Lamport++
	}
	msg.Timestamp = s.Lamport
}

// Function to remove the channel from the map after the client has left
func (s *AuctionServiceServer) removeChannel(ch *pb.Channel, currClientChannel chan *pb.Message) {
	channels := s.channel[ch.Name]
	for i, channel := range channels {
		if channel == currClientChannel {
			s.channel[ch.Name] = append(channels[:i], channels[i+1:]...)
			break
		}
	}
}

// Function to send message to all clients in the channel
func (s *AuctionServiceServer) sendMsgToClients(msg *pb.Message) {
	s.incrLamport(msg)

	go func() {
		if msg.Message == "9cbf281b855e41b4ad9f97707efdd29d" {
			msg.Message = fmt.Sprintf("Participant %v joined the Auction at Lamport time %v", msg.GetSender(), msg.GetTimestamp()-2)

			print := fmt.Sprintf("Received at Lamport time %v: %v", msg.GetTimestamp(), msg.GetMessage())
			fmt.Println(print)
			log.Println(print)
		} else {
			formattedMessage := fmt.Sprintf("Received at " + formatMessage(msg))
			log.Println(formattedMessage)
			fmt.Println(formattedMessage)
		}

		streams := s.channel[msg.Channel.Name]
		for _, clientChan := range streams {
			clientChan <- msg
		}
	}()
}

// Function to format message to be printed to the server
func formatMessage(msg *pb.Message) string {
	return fmt.Sprintf("Lamport time: %v [%v]: %v", msg.GetTimestamp(), msg.GetSender(), msg.GetMessage())
}

var randomInt, err = rand.Int(rand.Reader, big.NewInt(80))
var serverPort = flag.String("port", fmt.Sprintf(":80%v", randomInt), "The server port")

func main() {
	// Sets the logger to use a log.txt file instead of the console
	f := setLog()
	defer f.Close()

	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":80%v", randomInt))

	if err != nil {
		print := fmt.Sprintf("Failed to listen on port %v: %v", serverPort, err)
		log.Fatalf(print)
		fmt.Printf(print)
	}

	fmt.Println("--- EEPY AUCTION ---")

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterAuctionServiceServer(grpcServer, &AuctionServiceServer{
		channel: make(map[string][]chan *pb.Message),
		//Remote timestamp
		Lamport: 0,
	})

	print := fmt.Sprintf("Server started at port %v Lamport time: %v\n", lis.Addr().String(), 0)
	fmt.Printf(print)
	log.Printf(print)

	grpcServer.Serve(lis)
}

// sets the logger to use a log.txt file instead of the console
func setLog() *os.File {
	// Clears the log.txt file when a new server is started
	if _, err := os.Open("Server.txt"); err == nil {
		if err := os.Truncate("Server.txt", 0); err != nil {
			log.Printf("Failed to truncate: %v", err)
		}
	}

	// This connects to the log file/changes the output of the log information to the log.txt file.
	f, err := os.OpenFile("Server.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}
