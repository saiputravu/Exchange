.PHONY: cmd 

cmd: server client

server:
	mkdir -p ./build
	go build -o ./build/server ./cmd/server

client:
	mkdir -p ./build
	go build -o ./build/client ./cmd/client

clean:
	rm -rf ./build
