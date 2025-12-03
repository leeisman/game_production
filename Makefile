.PHONY: run-color-monolith run-color-robot build-color-monolith build-color-robot clean

# Default target
all: build-color-monolith build-color-robot

# Run Color Game Monolith
run-color-monolith:
	@echo "🚀 Starting Color Game Monolith..."
	@go run cmd/color_game/monolith/main.go

# Run Color Game Test Robot
# Usage: make run-color-robot USERS=1000
USERS ?= 8500
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
