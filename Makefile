BINARY := bin/heft
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test run clean work-on-fizzy

ifeq ($(firstword $(MAKECMDGOALS)),work-on-fizzy)
ifneq ($(words $(MAKECMDGOALS)),2)
$(error Usage: make work-on-fizzy <card_number>)
endif
FIZZY_CARD_NUMBER := $(word 2,$(MAKECMDGOALS))
.PHONY: $(FIZZY_CARD_NUMBER)
$(FIZZY_CARD_NUMBER):
	@:
endif

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/heft

install: build
	install -Dm755 $(BINARY) $(HOME)/.local/bin/heft

test:
	go test ./...

run:
	go run ./cmd/heft

clean:
	rm -rf bin

work-on-fizzy:
	@case "$(FIZZY_CARD_NUMBER)" in *[!0-9]*) echo "Usage: make work-on-fizzy <card_number>" >&2; exit 2;; esac
	heft work fizzy-$(FIZZY_CARD_NUMBER) --prompt="Check the fizzy card id=$(FIZZY_CARD_NUMBER), analyze it and prepare solution plan"
