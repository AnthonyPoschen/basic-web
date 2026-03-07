install_air:
	go install github.com/air-verse/air@latest
run:
	air --build.cmd "true" --build.bin "" --build.full_bin "go run main.go" --build.include_ext "go"
