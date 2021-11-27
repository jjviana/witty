
# cross compile cmd/witty/witty for Linux and MacOs (golang)
.PHONY :cross-compile
cross-compile:
	@echo "cross compiling cmd/witty/witty for Linux,MacOs and FreeBSD"
	mkdir -p build/witty-linux-amd64
	mkdir -p build/witty-darwin-amd64
	mkdir -p build/witty-freebsd-amd64
	GOOS=linux GOARCH=amd64 go build -o build/witty-linux-amd64/witty ./cmd/witty
	GOOS=darwin GOARCH=amd64 go build -o build/witty-darwin-amd64/witty ./cmd/witty
	GOOS=freebsd GOARCH=amd64 go build -o build/witty-freebsd-amd64/witty ./cmd/witty

# zip binaries
.PHONY :zip
zip:
	@echo "zipping binaries"
	cd build && zip -r witty-linux-amd64.zip witty-linux-amd64 && zip -r witty-darwin-amd64.zip witty-darwin-amd64 && zip -r witty-freebsd-amd64.zip witty-freebsd-amd64 

# get the current directory
pwd:=${PWD}
# run witty on Linux using Docker, inheriting the environment variable OPENAPI_API_KEY
.PHONY :run-witty-linux
run-witty-linux: build-witty-linux-run
	@echo "running witty on Linux"
	docker run -it --rm -v ${pwd}/build/witty-linux-amd64:/witty -e OPENAPI_API_KEY=${OPENAPI_API_KEY} witty-linux-run  bash -c 'cd /witty && ./witty'

# build the witty-linux-run image from the docker directory
.PHONY :build-witty-linux-run
build-witty-linux-run:
	@echo "building witty-linux-run image"
	cd docker && docker build -t witty-linux-run .


