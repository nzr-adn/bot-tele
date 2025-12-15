APP_NAME=bot-tele
MAIN_FILE=main.go

.PHONY: run build start stop clean

run:
	go run $(MAIN_FILE)

build:
	go build -o $(APP_NAME)

start: build
	./$(APP_NAME)

clean:
	rm -f $(APP_NAME)
