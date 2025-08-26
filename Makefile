PEER=7777
PORT=7777


#builds and run app
run:
	@go run ./cmd/p2poker -listen :${PORT}

peer:
	@go run ./cmd/p2poker -listen :${PORT} -peer localhost:${PEER}

#single-process in-proc mode (no sockets)
inproc:
	@go run ./cmd/p2poker -inproc