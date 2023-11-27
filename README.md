# Replication
This is the repository for team Eepy's mandatory handin 5: replication, in the course Distributed System 2023, at the IT-University of Copenhagen.

eepy auction service.

To use and run the Eepy Auction:

1. Open four seperate terminals.
2. In the first three terminals, start each of the servers by running the command 'go run server.go -port \<PortNumber\>', where PortNumber should be above the number 8000. 
3. In the last terminal, start a client/bidder by running the command 'go run client.go'.
4. Within the client, bid by simply typing the number you want to bid: \<integer\>, in the terminal, and press enter.
5. To let a client query the current auction, type the command '/r' into the client terminal.
6. To simulate any of the servers crashing, enter a server terminal, and press CRTL + C.
7. Log files for the users can be found in the client folder saved as \<username\> or Anon\<RandomInteger\>.txt.
8. Log files for the server can be found in the server folder saved as Server0x00000\<MemoryAddress\>.txt, the address here simulating the port it has opened. To see which port belongs to the specific txt file, inspect the first line of the .txt file, which will have the text 'Server started at port [::]:\<PortNumber\>', PortNumber corresponding to the given port the server ran on.


(sidenote: The servers port is set to 8080 in the code always so if its in use or blocked the server wont work unless the port is changed in the code)