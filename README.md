# p2p-poker (ongoing)

## Usage so far
- terminal 1: 
1. `make run`
2. `create dev 5 10 200` -> `created: t-AAAAAA` copy t-AAAAAA
3. `join t-AAAAAA`
- terminal 2: 
1. `make peer PORT=7778` (default = 7777)
2. `discover t-AAAAAA`
3. `join t-AAAAAA`
