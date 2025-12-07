package admin

import (
	"bytes"
	"context"
	"fmt"
	"os"
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

// CollectPerformanceData captures performance metrics
func (s *Server) CollectPerformanceData(ctx context.Context, req *pb.CollectReq) (*pb.CollectResp, error) {
	duration := time.Duration(req.DurationSeconds) * time.Second
	if duration <= 0 {
		duration = 30 * time.Second
	}

	// Buffers to hold data
	var cpuBuf bytes.Buffer
	var traceBuf bytes.Buffer
	var heapBuf bytes.Buffer
	var goroutineBuf bytes.Buffer

	// 1. Start CPU Profiling
	if err := pprof.StartCPUProfile(&cpuBuf); err != nil {
		return nil, fmt.Errorf("could not start CPU profile: %v", err)
	}

	// 2. Start Tracing
	if err := trace.Start(&traceBuf); err != nil {
		// Stop CPU profile if trace fails
		pprof.StopCPUProfile()
		return nil, fmt.Errorf("could not start trace: %v", err)
	}

	// 3. Wait for duration
	// We use a select to handle context cancellation (client disconnect)
	select {
	case <-time.After(duration):
		// Completed successfully
	case <-ctx.Done():
		// Client cancelled
		pprof.StopCPUProfile()
		trace.Stop()
		return nil, ctx.Err()
	}

	// 4. Stop Profiling & Tracing
	pprof.StopCPUProfile()
	trace.Stop()

	// 5. Capture Heap Snapshot
	if err := pprof.WriteHeapProfile(&heapBuf); err != nil {
		return nil, fmt.Errorf("could not write heap profile: %v", err)
	}

	// 6. Capture Goroutine Dump
	if p := pprof.Lookup("goroutine"); p != nil {
		if err := p.WriteTo(&goroutineBuf, 1); err != nil { // 1 = debug level (human readable?) but here we want binary/standard format?
			// Actually WriteTo writes text for goroutine if debug > 0.
			// Ideally we want stack traces. debug=1 is text. debug=2 is stack trace. 
			// Standard pprof tool expects binary usually, but for goroutines strictly speaking it parses text too?
			// Let's use debug=0 for binary proto format if supported, but Lookup.WriteTo might not support proto format for all profiles.
			// For "goroutine", debug=0 is actually the binary encoded profile suitable for `go tool pprof`.
			if err := p.WriteTo(&goroutineBuf, 0); err != nil {
				return nil, fmt.Errorf("could not write goroutine profile: %v", err)
			}
		}
	}

	// Get Hostname/Service Name (Simple approach)
	hostname, _ := os.Hostname()

	return &pb.CollectResp{
		CpuProfile:    cpuBuf.Bytes(),
		TraceData:     traceBuf.Bytes(),
		HeapSnapshot:  heapBuf.Bytes(),
		GoroutineDump: goroutineBuf.Bytes(),
		Timestamp:     time.Now().Unix(),
		ServiceName:   hostname,
	}, nil
}
