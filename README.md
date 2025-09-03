# P2Poker (ongoing)

“Every packet is a card.
Every node is a dealer.
The network is the table.”

P2Poker is built for the mesh underground — games that don’t need a lobby, don’t phone home, don’t care if you’re behind seven proxies and a dial-up modem. If you can send a TCP packet, you can play.

## Example usage

Here is an example of a full game between 2 players.\
Terminal1 will create a table, join the table, wait until someone else joins and start the game\
Terminal2 discovers and joins the created table

- Terminal1:
1. `make run`
2. `create dev 5 10 200` -> table t-AAAAAA created
3. `join t-AAAAAA`

- Terminal2:
4. `make peer PORT=7778`
5. `discover t-AAAAAA`
6. `join t-AAAAAA`

The table has 2+ players and is ready to start!\
Terminal1 `start t-AAAAAA`\
Commands include `board t-AAAAAA`, `state t-AAAAAA`, `whoami`\
Type `help` for all other commands
