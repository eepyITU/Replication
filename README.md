# Replication
This is the repository for team Eepy's mandatory handin 5: replication, in the course Distributed System 2023, at the IT-University of Copenhagen.

eepy auction service.

## How to run and use the Eepy Auction:
1. Open up as many terminals as you'd like. The client can connect to N many terminals. (No, we haven't limit-tested N.)
- We recommend opening two terminals for servers, and two terminals for clients. Or more if you want, you decide <3

2. For the server-terminals, start each of the servers by running the command `go run server/server.go -port <PortNumber>`.
- Note that running `go run server/server.go` defaults to port :8080.
- The auction has a predefined time-limit being 120 seconds. If you want it changed, add argument `-time <SecondsToClose>`.
- PS: The auction's time-limit only starts after the first bid has been registered by the server.
- PPS: If you set our own time-limit on one server, please do so on the other servers as well. For consistency. Y'know.

3. In the client-terminal(s), start a client/bidder by running the command `go run client/client.go -ports <PortNumber>,..,<PortNumber>`.
- Example: `go run client/client.go -ports 8080,8034,4040`
- If you want to set your own username, add argument `-username <Username>`. Else it will default to `Anon <RandomInt>`

4. Within the client, bid by simply typing the number you want to bid: `<Integer>`, in the terminal, and press enter.

5. To let a client query the current auction, type the command `/r` into the client terminal.

6. To simulate any of the servers crashing, enter a server-terminal, and press CRTL + C.

7. Log files for the users can be found in the client folder saved as <`Username>.txt` or `<Anon<RandomInteger>.txt.`

8. Log files for the server can be found in the server folder saved as `Server<PortNumber>.txt`.

## Copy-pastes for TA's convenience:
3 servers:
`go run server/server.go`
`go run server/server.go -port 8081`

2 clients:
`go run client/client.go -ports 8080,8081`
`go run client/client.go -username LovelyTA -ports 8080,8081`