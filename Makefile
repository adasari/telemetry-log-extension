build: clean
	@echo "Building the extension binary"
	go mod tidy
    GOOS=linux GOARCH=amd64 go build -o ./bin/extensions/lambda-telemetry-extension main.go
    @echo "Build complete."

clean:
	rm -rf bin
	rm -rf go.sum