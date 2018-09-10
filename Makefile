
main_sources=$(filter-out %_test.go,$(wildcard *.go))

run: game
	./$<

.PHONY: game
.PRECIOUS: game
game:
	go build -o $@ $(main_sources)

test:
	go test ./...
