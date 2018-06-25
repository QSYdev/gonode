build:
	env GOOS=darwin GOARCH=amd64 go build -o gonode-macos
	env GOOS=linux GOARCH=arm GOARM=5 go build -o gonode-pi
