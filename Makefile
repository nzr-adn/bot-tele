APP_NAME=bot-tele
MAIN_FILE=main.go
TMUX_SESSION=bot-tele

.PHONY: run build clean tmux-start tmux-attach tmux-stop tmux-status

run:
	go run $(MAIN_FILE)

build:
	go build -o $(APP_NAME)

clean:
	rm -f $(APP_NAME)

# ===== TMUX =====

tmux-start: build
	@tmux has-session -t $(TMUX_SESSION) 2>/dev/null || \
	tmux new -d -s $(TMUX_SESSION) "cd $(PWD) && ./$(APP_NAME)"

tmux-attach:
	tmux attach -t $(TMUX_SESSION)

tmux-stop:
	tmux kill-session -t $(TMUX_SESSION)

tmux-status:
	tmux ls
