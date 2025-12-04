#!/bin/bash
PID_FILE="color.pid"

if [ ! -f "$PID_FILE" ]; then
    echo "❌ color.pid not found. Is the daemon running?"
    exit 1
fi

PID=$(cat "$PID_FILE")
echo "🔍 Monitoring PID: $PID"
echo "Time       | CCU (Est.) | Load Avg (5m)"
echo "-----------+------------+---------------"

while true; do
    TIME=$(date +%H:%M:%S)
    # lsof is accurate but can be heavy with 13k conns. 
    # netstat might be faster for simple counting if lsof is too slow.
    CONNS=$(lsof -n -P -p $PID | grep TCP | wc -l)
    
    LOAD=$(sysctl -n vm.loadavg | awk '{print $3}')
    
    printf "%s | %10d | %13s\n" "$TIME" "$CONNS" "$LOAD"
    sleep 2
done
