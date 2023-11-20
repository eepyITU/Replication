package main

import (
	pb "Replication/AuctionSystem/Proto"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuctionServiceServer struct {
	pb.UnimplementedAuctionServiceServer
	Leadertoken    bool
	ClientChannels map[string][]chan *pb.Message
	ServerChannels map[string][]chan *pb.Message
	Lamport        int32
	HighestBid     int
	HighestBidder  string
}

func (s *AuctionServiceServer) JoinAuction(ch *pb.Channel, msgStream pb.AuctionService_JoinAuctionServer) error {
	clientChannel := make(chan *pb.Message)
	s.ClientChannels[ch.Name] = append(s.ClientChannels[ch.Name], clientChannel)

	for {
		select {
		case <-msgStream.Context().Done():
			leaveString := fmt.Sprintf("Client %v has left the Auction at %v", ch.GetUserName(), s.Lamport)

			s.removeChannel(ch, clientChannel)

			msg := &pb.Message{Sender: ch.Name, Message: leaveString, Channel: ch, Timestamp: s.Lamport}

			s.sendMsgToClients(msg)

			return nil
		case msg := <-clientChannel:
			msgStream.Send(msg)
		}
	}
}

func (s *AuctionServiceServer) removeChannel(ch *pb.Channel, currClientChannel chan *pb.Message) {
	channels := s.ClientChannels[ch.Name]
	for i, channel := range channels {
		if channel == currClientChannel {
			s.ClientChannels[ch.Name] = append(channels[:i], channels[i+1:]...)
			break
		}
	}
}

func (s *AuctionServiceServer) sendMsgToClients(msg *pb.Message) {
	s.Lamport++
	msg.Timestamp = s.Lamport

	go func() {
		formattedMessage := formatMessage(msg)
		fmt.Printf("Received at " + formattedMessage)

		streams := s.ClientChannels[msg.Channel.Name]
		for _, clientChan := range streams {
			clientChan <- msg
		}
	}()
}

func formatMessage(msg *pb.Message) string {
	return fmt.Sprintf("Lamport time: %v [%v]: %v\n",
		msg.GetTimestamp(), msg.GetSender(), msg.GetMessage())
}

func isNumeric(msg string) bool {
	var number int
	_, err := fmt.Sscanf(msg, "%d", &number)
	return err == nil
}

func (s *AuctionServiceServer) ProcessMessage(msgStream pb.AuctionService_ProcessMessageServer) error {
	msg, err := msgStream.Recv()

	s.incrLamport(msg)

	if err == io.EOF {
		return nil
	}

	if err != nil {
		return err
	}

	if msg.GetMessage() == "token" {
		s.ProcessTokenReq()
	} else if isNumeric(msg.GetMessage()) {
		s.ProcessBid(msg)
	}

	ack := pb.MessageAck{Status: "Sent"}
	msgStream.SendAndClose(&ack)

	s.sendMsgToClients(msg)

	return nil
}

func (s *AuctionServiceServer) incrLamport(msg *pb.Message) {
	if msg.GetTimestamp() > s.Lamport {
		s.Lamport = msg.GetTimestamp() + 1
	} else {
		s.Lamport++
	}
	msg.Timestamp = s.Lamport
}

func (s *AuctionServiceServer) ProcessBid(msg *pb.Message) {
	var bid int
	bid, _ = fmt.Sscanf(msg.Message, "%d")

	if bid > s.HighestBid {
		s.HighestBid = bid
		s.HighestBidder = msg.Sender
		// Then send these values to the backup servers
		// send acknowledgement that the bid was successful
	} else {
		// send acknowledgement that the bid was unsuccessful
		// send the highest bid amount to the client
	}
}

func (s *AuctionServiceServer) ProcessTokenReq() {

}

func GeneratePortNum() int {
	return (rand.Intn(1000) + 4000)
}

var serverPortNum = flag.Int("port", 1, "The port number of the server")

func main() {
	f := setLog()
	defer f.Close()

	flag.Parse()

	var lis net.Listener
	err := errors.New("Hasn't been initiated yet")

	if *serverPortNum == 1 {
		for err != nil && *serverPortNum < 100 {
			*serverPortNum = GeneratePortNum()
			log.Println("Attempting server startup on port:", *serverPortNum)
			serverPort := fmt.Sprintf(":%v", *serverPortNum)
			lis, err = net.Listen("tcp", serverPort)
		}
	}

	fmt.Println("--- EEPY AUCTION --")

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterAuctionServiceServer(grpcServer, &AuctionServiceServer{
		ClientChannels: make(map[string][]chan *pb.Message),
		ServerChannels: make(map[string][]chan *pb.Message),
		Lamport:        0,
		HighestBid:     0,
		HighestBidder:  "No one yet.",
	})

	print := fmt.Sprintf("Server-startup success on %v at Lamport time: %v\n", lis.Addr(), 0)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	} else {
		fmt.Print(print)
		log.Print(print)
	}

	// [ ] Add a way to ping the other servers to see if they are up and running

	count := 0
	waitg := sync.WaitGroup{}
	for count < 100 {
		address := fmt.Sprintf("localhost:%v", 4000+count)
		if time.Second < 30 && pingServer(&waitg, address) == true {
			s.ServerChannels[address] =	}
}

// [ ] Clean up this shit
func pingServer(wg *sync.WaitGroup, address string) bool {
	log.Println("Pinging server:", address)
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return true
	}

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Could not close connection")
		}
	}(conn)
}

func setLog() *os.File {
	// Clears the log.txt file when a new server is started
	// But... what if we have multiple servers running at the same time?
	_, err := os.Open("Server.txt")

	if err == nil {
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
