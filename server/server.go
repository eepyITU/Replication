package main

import (
	pb "Replication/proto"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
)

var resultCommand = "/r"
var timeLimitCountdownStarted = false
var timeLimitReached = false
var auctionStopsAt = time.Now().Local()
var serverCrashesAt = time.Now().Local()

type AuctionServiceServer struct {
	pb.UnimplementedAuctionServiceServer
	channel           map[string][]chan *pb.Message
	Lamport           int32
	CurrentHighestBid int32
	HighestBidder     string
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
			s.removeChannel(ch, clientChannel)
			return nil

		// if a client sends a message, incr! :D Since server has RECEIVED a msg
		case msg := <-clientChannel:
			log.Println("0: Auction stops at: " + auctionStopsAt.String())
			s.incrLamport(msg)

			log.Println("Message received from client: " + msg.GetMessage())

			// stream sends the message to SendMessage function
			msgStream.Send(msg)
			log.Println("1: Auction stops at: " + auctionStopsAt.String())
		}
	}
}

func (s *AuctionServiceServer) SendMessage(msgStream pb.AuctionService_SendMessageServer) error {
	setAuctionTimelimit()
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

	log.Println("3: Auction stops at: " + auctionStopsAt.Format("15:04:05"))
	if timeLimitReached == false {
		// Check if message is a result request, if its isnt it must be a bid.
		// This is possible as the client will only send a result req. or a bid.
		if msg.GetMessage() == resultCommand {
			statusString := fmt.Sprintf("Result. Current Bid: %v - Bidder: %v - Auction closes at %v", s.CurrentHighestBid, s.HighestBidder, auctionStopsAt.Format("15:04:05"))
			ack = pb.MessageAck{Status: statusString}
		} else {
			// check if message is integer && higher than current highest bid
			if s.validBid(msg.GetMessage()) {

				// Convert string to int32
				num, err := strconv.ParseInt(msg.GetMessage(), 10, 32)
				if err != nil {
					log.Fatalf("Error parsing bid to int32: %v", err)
				}
				// Set new highest bid and bidder
				s.CurrentHighestBid = int32(num)
				s.HighestBidder = msg.GetSender()

				statusString := fmt.Sprintf("Accepted: %v - Auction closes at %v", msg.GetMessage(), auctionStopsAt.Format("15:04:05"))
				ack = pb.MessageAck{Status: statusString}
			} else {
				statusString := fmt.Sprintf("Rejected. Current Bid: %v - Auction closes at %v", s.CurrentHighestBid, auctionStopsAt.Format("15:04:05"))
				ack = pb.MessageAck{
					Status: statusString,
					Bid:    s.CurrentHighestBid,
				}
			}
		}
	} else {
		statusString := fmt.Sprintf("The auction has ended - Winning Bid: %v - Bid By: %v", s.CurrentHighestBid, s.HighestBidder)
		ack = pb.MessageAck{Status: statusString}
	}

	// Acknowledge message received to client
	msgStream.SendAndClose(&ack)

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

func setAuctionTimelimit() {

	log.Println()

	if timeLimitCountdownStarted == false {
		timeLimitCountdownStarted = true

		auctionStopsAt = time.Now().Local().Add(time.Duration(*timeLimitDuration) * time.Second)
	} else {
		if time.Now().Local().After(auctionStopsAt) {
			timeLimitReached = true
			print := "Auction time limit reached."
			fmt.Println(print)
			log.Println(print)
		}
	}
}

var serverPort = flag.Int("port", 8080, "The server port")
var timeLimitDuration = flag.Int("time", 120, "The auction time limit in seconds")

func main() {
	// Sets the logger to use a log.txt file instead of the console
	flag.Parse()
	f := setLog(*serverPort)
	defer f.Close()

	connectionString := ":" + strconv.Itoa(*serverPort)
	lis, err := net.Listen("tcp", connectionString)

	if err != nil {
		print := fmt.Sprintf("Failed to listen on port %v: %v", connectionString, err)
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

	print := fmt.Sprintf("Server started at port %v Lamport time: %v\nTo simulate a crash, please press CTRL + C", lis.Addr().String(), 0)
	fmt.Print(print)
	log.Print(print)
	grpcServer.Serve(lis)
}

// sets the logger to use a log.txt file instead of the console
func setLog(serverPort int) *os.File {
	serverTextName := fmt.Sprintf("Server%v.txt", serverPort)
	if _, err := os.Open(serverTextName); err == nil {
		if err := os.Truncate(serverTextName, 0); err != nil {
			log.Printf("Failed to truncate: %v", err)
		}
	}

	// This connects to the log file/changes the output of the log information to the log.txt file.
	f, err := os.OpenFile(serverTextName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}
