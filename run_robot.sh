#!/bin/bash

# Build the robot
echo "Building test_robot..."
go build -o test_robot cmd/color_game/test_robot/main.go

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Run the robot
echo "Starting test_robot..."
./test_robot "$@"
