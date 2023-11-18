package main

import (
	pb "Replication/AuctionSystem/Proto"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
)

type AuctionServiceServer struct {
	pb.UnimplementedAuctionServiceServer
	Leadertoken    bool
	ServerId       int
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

var serverId int = *flag.Int("id", 0, "server id")
var serverPort string = *flag.String("port", ":808", "server port")

func main() {
	f := setLog()
	defer f.Close()

	flag.Parse()

	leadertoken := serverId == 0
	serverPort = fmt.Sprintf(":%v", 8080+serverId)

	lis, err := net.Listen("tcp", serverPort)

	if err != nil {
		log.Fatalf("Failed to listen on port %v: %v", serverPort, err)
	}

	fmt.Println("--- EEPY AUCTION --")

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterAuctionServiceServer(grpcServer, &AuctionServiceServer{
		ServerId:       serverId,
		Leadertoken:    leadertoken,
		ServerChannels: make(map[string][]chan *pb.Message),
		ClientChannels: make(map[string][]chan *pb.Message),
		Lamport:        0,
	})

	print := fmt.Sprintf("Server, %v started at Lamport time: %v\n", serverId, 0)

	fmt.Print(print)
	log.Print(print)

	grpcServer.Serve(lis)
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
