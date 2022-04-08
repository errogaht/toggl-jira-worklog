install_inner:
	cp -f tjwl ~/.local/bin/
install: build install_inner
build:
	go build -o tjwl app.go

run:
	go run app.go