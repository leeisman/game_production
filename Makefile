.PHONY: run-color-monolith run-color-robot build-color-monolith build-color-robot clean

# Default target
all: build-color-monolith build-color-robot

# Run Color Game Monolith
run-color-monolith:
	@echo "🚀 Starting Color Game Monolith..."
	@go run cmd/color_game/monolith/main.go

# Run Color Game Monolith with pprof enabled
run-color-monolith-pprof:
	@echo "🚀 Starting Color Game Monolith with pprof on :6060..."
	@go run cmd/color_game/monolith/main.go -pprof-port=6060

# Run in background (daemon mode)
run-color-daemon:
	@echo "🚀 Starting Color Game Monolith in daemon mode..."
	@nohup go run cmd/color_game/monolith/main.go -d > logs/color_game/monolith.out 2>&1 & echo $$! > color.pid
	@echo "✅ Started with PID $$(cat color.pid). Logs: logs/color_game/monolith.log"

# Stop daemon process
stop-color-daemon:
	@echo "🛑 Stopping daemon process..."
	@kill $$(cat color.pid) && rm color.pid
	@echo "✅ Stopped."

# Monitor OS Network I/O (PPS/Bandwidth)
monitor-io:
	@echo "📊 Monitoring Network I/O (Press Ctrl+C to stop)..."
	@echo "Look at 'packets' column. If it plateaus, you hit the limit."
	@netstat -w 1

# Monitor Connections and Load
monitor-conns:
	@./scripts/monitor_conns.sh
run-color-robot:
	@echo "🤖 Starting Color Game Test Robot with $(USERS) users..."
	@go run cmd/color_game/test_robot/main.go -users $(USERS) -log-level info

# Build Binaries
build-color-monolith:
	@echo "🔨 Building Color Game Monolith..."
	@go build -o bin/color_game_monolith cmd/color_game/monolith/main.go

build-color-robot:
	@echo "🔨 Building Color Game Test Robot..."
	@go build -o bin/color_game_robot cmd/color_game/test_robot/main.go

# Clean build artifacts and logs
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@rm -rf logs/
	@rm -f trace.out

# pprof Analysis Tools
# Usage: make pprof-cpu PORT=6060 SECONDS=30
PORT ?= 6060
SECONDS ?= 30

pprof-cpu:
	@echo "🔥 Capturing CPU profile for $(SECONDS) seconds..."
	@go tool pprof -http=:9081 http://localhost:$(PORT)/debug/pprof/profile?seconds=$(SECONDS)

pprof-heap:
	@echo "💾 Capturing Heap profile..."
	@go tool pprof -http=:9082 http://localhost:$(PORT)/debug/pprof/heap

pprof-trace:
	@echo "🕵️ Capturing Execution Trace for 5 seconds..."
	@curl -o trace.out http://localhost:$(PORT)/debug/pprof/trace?seconds=5
	@go tool trace trace.out
