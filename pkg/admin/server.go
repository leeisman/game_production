package admin

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"time"

	pb "github.com/frankieli/game_product/shared/proto/admin"
)

// Server implements the AdminServiceServer interface
type Server struct {
	pb.UnimplementedAdminServiceServer
}

// NewServer creates a new Admin Server
func NewServer() *Server {
	return &Server{}
}

// CollectPerformanceData captures performance metrics and streams them back
func (s *Server) CollectPerformanceData(req *pb.CollectReq, stream pb.AdminService_CollectPerformanceDataServer) error {
	duration := time.Duration(req.DurationSeconds) * time.Second
	if duration <= 0 {
		duration = 30 * time.Second
	}

	// Buffers to hold data
	var cpuBuf bytes.Buffer
	var traceBuf bytes.Buffer
	var heapBuf bytes.Buffer
	var goroutineBuf bytes.Buffer
	var blockBuf bytes.Buffer // New: IO/Blocking
	var mutexBuf bytes.Buffer // New: Lock Contention

	// A. Enable Profiles (Block & Mutex are disabled by default)
	// 1ns = Capture everything (High overhead, perfect for debugging)
	runtime.SetBlockProfileRate(1)
	// 1 = Capture every contention event (Default is 0)
	runtime.SetMutexProfileFraction(1)

	// Defer: RESTORE to 0 (Disable) to prevent production performance hit
	defer func() {
		runtime.SetBlockProfileRate(0)
		runtime.SetMutexProfileFraction(0)
	}()

	// 1. Start CPU Profiling
	if err := pprof.StartCPUProfile(&cpuBuf); err != nil {
		return fmt.Errorf("could not start CPU profile: %v", err)
	}

	// 2. Start Tracing
	if err := trace.Start(&traceBuf); err != nil {
		pprof.StopCPUProfile()
		return fmt.Errorf("could not start trace: %v", err)
	}

	// 3. Wait for duration
	ctx := stream.Context()
	select {
	case <-time.After(duration):
		// Completed successfully
	case <-ctx.Done():
		// Client cancelled
		pprof.StopCPUProfile()
		trace.Stop()
		return ctx.Err()
	}

	// 4. Stop Profiling & Tracing
	pprof.StopCPUProfile()
	trace.Stop()

	// 5. Capture Snapshots
	if err := pprof.WriteHeapProfile(&heapBuf); err != nil {
		return fmt.Errorf("could not write heap profile: %v", err)
	}
	if p := pprof.Lookup("goroutine"); p != nil {
		p.WriteTo(&goroutineBuf, 0)
	}

	// New: Capture Block Profile (IO Wait)
	if p := pprof.Lookup("block"); p != nil {
		p.WriteTo(&blockBuf, 0)
	}
	// New: Capture Mutex Profile (Lock Wait)
	if p := pprof.Lookup("mutex"); p != nil {
		p.WriteTo(&mutexBuf, 0)
	}

	hostname, _ := os.Hostname()
	timestamp := time.Now().Unix()

	// Helper to send data in chunks
	sendData := func(dataType pb.CollectRespChunk_DataType, data []byte) error {
		const chunkSize = 32 * 1024
		for i := 0; i < len(data); i += chunkSize {
			end := i + chunkSize
			if end > len(data) {
				end = len(data)
			}
			chunk := &pb.CollectRespChunk{
				DataType:    dataType,
				Data:        data[i:end],
				Timestamp:   timestamp,
				ServiceName: hostname,
			}
			if err := stream.Send(chunk); err != nil {
				return err
			}
		}
		return nil
	}

	// Send all data types
	if err := sendData(pb.CollectRespChunk_CPU_PROFILE, cpuBuf.Bytes()); err != nil {
		return err
	}
	if err := sendData(pb.CollectRespChunk_TRACE_DATA, traceBuf.Bytes()); err != nil {
		return err
	}
	if err := sendData(pb.CollectRespChunk_HEAP_SNAPSHOT, heapBuf.Bytes()); err != nil {
		return err
	}
	if err := sendData(pb.CollectRespChunk_GOROUTINE_DUMP, goroutineBuf.Bytes()); err != nil {
		return err
	}

	// Send New Profiles
	if err := sendData(pb.CollectRespChunk_BLOCK_PROFILE, blockBuf.Bytes()); err != nil {
		return err
	}
	if err := sendData(pb.CollectRespChunk_MUTEX_PROFILE, mutexBuf.Bytes()); err != nil {
		return err
	}

	return nil
}
