MAIN := ./cmd/nirilayout
SRCS := $(wildcard *.go cmd/nirilayout/*.go) go.mod go.sum style.css

nirilayout: $(SRCS)
	go build -o $@ $(MAIN)

nirilayout-profile: $(SRCS)
	go build -o $@ -tags profile $(MAIN)

install: $(SRCS)
	go install $(MAIN)

clean:
	rm -f nirilayout nirilayout-profile

.PHONY: clean
