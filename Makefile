BINARY   := noor
INSTALL  := $(HOME)/.local/bin/$(BINARY)

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY) $(INSTALL)
	@echo "Installed → $(INSTALL)"

uninstall:
	rm -f $(INSTALL)
	@echo "Removed $(INSTALL)"

clean:
	rm -f $(BINARY)

.PHONY: build install uninstall clean
