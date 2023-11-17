package main

import (
	pb "Replication/AuctionSystem/Proto"
	"fmt"
	"io"
	"log"
	"os"
)

type AuctionServiceServer struct {
	pb.UnimplementedAuctionServiceServer
	Leadertoken      bool
	ClientChannels   map[string][]chan *pb.Message
	ServerChannels   map[string][]chan *pb.Message
	Lamport          int32
	HighestBidAmount int32
	HighestBidder    string
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

func (s *AuctionServiceServer) ProcessMessage(msgStream pb.AuctionService_ProcessMessageServer) error {
	msg, err := msgStream.Recv()

	s.incrLamport(msg)

	if err == io.EOF {
		return nil
	}

	if err != nil {
		return err
	}

	ack := pb.MessageAck{Status: "Sent"}
	msgStream.SendAndClose(&ack)

	s.sendMsgToClients()

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
	if msg.Message {
		s.HighestBidAmount = msg.Message
		s.HighestBidder = msg.Sender
	}
}

func (s *AuctionServiceServer) ProcessTokenReq() {

}

func main() {

}

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
