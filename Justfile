tidy:
	# Needed to fetch new dependencies and add them to go.mod
	go mod tidy

fmt:
    # Formats the code
    go fmt ./...
