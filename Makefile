.PHONY: boot test lint fmt clean

boot:
	docker compose up --build

test:
	go test -race ./...

lint:
	gofmt -l .
	staticcheck ./... 2>/dev/null || true

fmt:
	gofmt -w -s .

clean:
	rm -rf cmd/*/server cmd/*/worker-sim
