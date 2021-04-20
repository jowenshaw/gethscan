.PHONY: all clean fmt

GOBIN = ./build/bin
GOCMD = env GO111MODULE=on GOPROXY=https://goproxy.io go

all:
	$(GOCMD) build -v -o $(GOBIN)/gethscan
	@echo "Done building."
	@echo "Find binaries in \"$(GOBIN)\" directory."
	@echo ""
	@echo "Copy config-example.toml to \"$(GOBIN)\" directory"
	@cp -uv params/config-example.toml $(GOBIN)

clean:
	$(GOCMD) clean -cache
	rm -fr $(GOBIN)/*

fmt:
	./gofmt.sh
