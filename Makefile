BINDIR := bin

.PHONY: build clean

build:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/hyve ./cmd/hyve
	go build -o $(BINDIR)/hyved ./cmd/hyved

clean:
	rm -rf $(BINDIR)