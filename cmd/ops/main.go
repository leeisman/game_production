package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/grpc_client/base"
	"github.com/frankieli/game_product/pkg/grpc_client/color_game"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/gin-gonic/gin"

	// Proto definitions for unmarshalling
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

//go:embed frontend/dist/*
var staticFS embed.FS

// --- Multi-Project Structure ---

type ProjectConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NacosHost string `json:"nacos_host"`
	NacosPort string `json:"nacos_port"`
	Namespace string `json:"namespace"`
}

type ProjectContext struct {
	Config   ProjectConfig
	Registry discovery.Registry
	CGClient *color_game.Client // Contains BaseClient internally
}

// Global Registry
var (
	projects       = make(map[string]*ProjectContext)
	projectList    = []ProjectConfig{} // For frontend listing
	methodRegistry map[string]GenericHandler
)

// GenericHandler now accepts the project context
type GenericHandler func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error)

func main() {
	// 1. Initialize Global Logger
	logger.InitWithFile("logs/ops/server.log", "info", "json", true)
	logger.InfoGlobal().Msg("🚀 Starting OPS Center Backend...")

	// 2. Initialize Projects & Clients
	initProjects()
	initRegistry()
	startPprofCleanupMonitor()

	// 3. Setup Gin Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 3. Serve Frontend Assets
	r.StaticFS("/assets", http.FS(mustSub(staticFS, "frontend/dist/assets")))
	r.GET("/favicon.ico", func(c *gin.Context) {
		data, err := staticFS.ReadFile("frontend/dist/favicon.ico")
		if err != nil {
			c.Status(404)
			return
		}
		c.Data(200, "image/x-icon", data)
	})

	// 4. API Routes
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// List available projects
		api.GET("/projects", func(c *gin.Context) {
			c.JSON(200, projectList)
		})

		// List Services (requires ?project=xxx)
		api.GET("/services", handleListServices)

		// RPC Call (payload contains project)
		api.POST("/grpc_call", handleGenericCall)

		// Performance Profiling
		api.POST("/performance/record", handleRecordPerformance)
		api.GET("/performance/history", handleListPerformanceHistory)
		api.DELETE("/performance/history", handleDeletePerformanceHistory)

		// Full Web UI Proxy (go tool pprof -http)
		// /api/performance/ui/:key/*path
		api.Any("/performance/ui/:key/*path", handlePprofProxy)

		// Serve downloaded profiles
		api.Static("/performance/download", "./storage/pprof")
	}

	// 5. SPA Catch-all
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		data, err := staticFS.ReadFile("frontend/dist/index.html")
		if err != nil {
			c.String(404, "Index not found")
			return
		}
		c.Data(200, "text/html; charset=utf-8", data)
	})

	port := getEnv("OPS_PORT", "8080")
	logger.InfoGlobal().Msgf("🚀 OPS Server running at :%s", port)
	if err := r.Run(":" + port); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to start server")
	}
}

func initProjects() {
	// Define Game Projects here
	// Each game maps to a specific Nacos Namespace/Cluster
	configs := []ProjectConfig{
		{
			ID:        "color_game",
			Name:      "Color Game",
			NacosHost: getEnv("NACOS_HOST", "localhost"),
			NacosPort: getEnv("NACOS_PORT", "8848"),
			Namespace: getEnv("NACOS_NAMESPACE", "public"),
		},
		// Future games can be added here
		// { ID: "poker", Name: "Poker", ... Namespace: "poker-ns" },
	}

	for _, cfg := range configs {
		logger.InfoGlobal().Msgf("Initializing Project: %s (%s)", cfg.Name, cfg.Namespace)

		// 1. Nacos
		registry, err := discovery.NewNacosClient(cfg.NacosHost, cfg.NacosPort, cfg.Namespace)
		if err != nil {
			logger.ErrorGlobal().Err(err).Msgf("[%s] Failed to create Nacos client", cfg.ID)
			continue
		}

		// 2. Color Game Client (which contains BaseClient internally)
		cgClient, err := color_game.NewClient(base.NewBaseClient(registry))
		if err != nil {
			logger.WarnGlobal().Err(err).Msgf("[%s] Failed to create ColorGame client", cfg.ID)
		}

		ctx := &ProjectContext{
			Config:   cfg,
			Registry: registry,
			CGClient: cgClient,
		}

		projects[cfg.ID] = ctx
		projectList = append(projectList, cfg)
	}
}

func getProjectContext(id string) (*ProjectContext, error) {
	if p, ok := projects[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("project '%s' not found", id)
}

// --- Handlers ---

func handleListServices(c *gin.Context) {
	projectID := c.Query("project")
	if projectID == "" {
		// Default to first available if none specified? Or error.
		if len(projectList) > 0 {
			projectID = projectList[0].ID
		}
	}

	p, err := getProjectContext(projectID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	nacosClient, ok := p.Registry.(*discovery.NacosClient)
	if !ok {
		c.JSON(500, gin.H{"error": "Registry is not Nacos"})
		return
	}

	services, err := nacosClient.GetAllServicesList()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	type ServiceInfo struct {
		Name      string   `json:"name"`
		Instances []string `json:"instances"`
		Count     int      `json:"count"`
	}
	result := []ServiceInfo{}

	for _, s := range services {
		// Use Registry directly to get healthy instances
		addrs, _ := p.Registry.GetServices(s)
		result = append(result, ServiceInfo{
			Name:      s,
			Instances: addrs,
			Count:     len(addrs),
		})
	}
	c.JSON(200, result)
}

func handleGenericCall(c *gin.Context) {
	var body struct {
		Project string          `json:"project"`
		Method  string          `json:"method"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := c.BindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request format"})
		return
	}

	// 1. Get Project Context
	p, err := getProjectContext(body.Project)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 2. Find Handler
	handler, exists := methodRegistry[body.Method]
	if !exists {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Method '%s' not found", body.Method)})
		return
	}

	// 3. Execute
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := handler(ctx, p, []byte(body.Payload))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

func handleRecordPerformance(c *gin.Context) {
	var body struct {
		Project  string `json:"project"`
		Service  string `json:"service"`
		Instance string `json:"instance"` // Optional: specific ip:port
		Duration int32  `json:"duration"`
	}

	if err := c.BindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	p, err := getProjectContext(body.Project)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Default duration 30s
	if body.Duration <= 0 {
		body.Duration = 30
	}
	if body.Duration > 600 { // Max 10 mins
		body.Duration = 600
	}

	// Call RPC (long running)
	// Timeout should be duration + buffer (e.g. 10s overhead)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(body.Duration+10)*time.Second)
	defer cancel()

	logger.InfoGlobal().Msgf("Starting performance collection for %s (Inst: %s) duration %ds", body.Service, body.Instance, body.Duration)

	resp, err := p.CGClient.CollectPerformance(ctx, body.Service, body.Instance, body.Duration)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("CollectPerformance RPC failed")
		c.JSON(500, gin.H{"error": fmt.Sprintf("RPC failed: %v", err)})
		return
	}

	// Save files
	timestamp := time.Now().Unix()
	// Sanitize Instance IP for folder name (replace : with -)
	safeInstance := strings.ReplaceAll(body.Instance, ":", "-")
	if safeInstance == "" {
		safeInstance = "unknown"
	}

	// Folder format: {timestamp}__{service}__{instance}
	baseDir := fmt.Sprintf("./storage/pprof/%d__%s__%s", timestamp, body.Service, safeInstance)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create storage dir"})
		return
	}

	// Helper to write file
	writeFile := func(name string, data []byte) string {
		if len(data) == 0 {
			return ""
		}
		path := filepath.Join(baseDir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			logger.WarnGlobal().Err(err).Msgf("Failed to write file %s", name)
			return ""
		}
		// Return relative path for download URL: "timestamp__service__instance/filename"
		return fmt.Sprintf("%d__%s__%s/%s", timestamp, body.Service, safeInstance, name)
	}

	files := gin.H{
		"cpu":       writeFile("cpu.prof", resp.CpuProfile),
		"trace":     writeFile("trace.out", resp.TraceData),
		"heap":      writeFile("heap.prof", resp.HeapSnapshot),
		"goroutine": writeFile("goroutine.prof", resp.GoroutineDump),
	}

	c.JSON(200, gin.H{
		"success":      true,
		"timestamp":    timestamp,
		"service_name": body.Service,
		"instance":     body.Instance,
		"files":        files,
	})
}

func handleListPerformanceHistory(c *gin.Context) {
	baseDir := "./storage/pprof"
	// Ensure dir exists
	os.MkdirAll(baseDir, 0o755)

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to list directory"})
		return
	}

	type HistoryItem struct {
		Folder      string `json:"folder"`
		Timestamp   int64  `json:"timestamp"`
		ServiceName string `json:"service_name"`
		Instance    string `json:"instance"`
		Files       gin.H  `json:"files"`
	}
	history := []HistoryItem{}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Parse name: "TIMESTAMP__SERVICE__INSTANCE"
		// or legacy "TIMESTAMP__SERVICE"
		parts := strings.Split(name, "__")
		if len(parts) < 2 {
			continue
		}
		ts, _ := strconv.ParseInt(parts[0], 10, 64)
		service := parts[1]
		instance := "unknown"
		if len(parts) >= 3 {
			instance = strings.ReplaceAll(parts[2], "-", ":")
		}

		// Check files inside
		files := gin.H{}
		subEntries, _ := os.ReadDir(filepath.Join(baseDir, name))
		for _, f := range subEntries {
			if !f.IsDir() {
				// e.g. "cpu.prof" -> url: "folder/cpu.prof"
				key := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())) // "cpu"
				// Handle "trace.out"
				if f.Name() == "trace.out" {
					key = "trace"
				}
				files[key] = fmt.Sprintf("%s/%s", name, f.Name())
			}
		}

		history = append(history, HistoryItem{
			Folder:      name,
			Timestamp:   ts,
			ServiceName: service,
			Instance:    instance,
			Files:       files,
		})
	}

	c.JSON(200, history)
}

// --- Pprof Proxy Manager ---

type PprofSession struct {
	Port       int
	Cmd        *exec.Cmd
	LastAccess time.Time
}

var (
	pprofSessions = make(map[string]*PprofSession) // Key: "folder/filename"
	pprofMutex    sync.Mutex
)

// GetOrStartPprofSession returns the port for a pprof web server
func GetOrStartPprofSession(targetPath string) (int, error) {
	pprofMutex.Lock()
	defer pprofMutex.Unlock()

	// 1. cleaning up old sessions (mock LRU: random cleanup if too many?)
	// For now simple cleanup: if process died, remove it.
	for k, s := range pprofSessions {
		if s.Cmd.ProcessState != nil && s.Cmd.ProcessState.Exited() {
			delete(pprofSessions, k)
		}
	}

	// 2. Check existing
	if s, ok := pprofSessions[targetPath]; ok {
		s.LastAccess = time.Now()
		// Check if port is still listening?
		// Assuming yes if process is running.
		return s.Port, nil
	}

	// 3. LRU Eviction: Limit concurrent sessions
	// User requested "close previous on new open", so we keep this limit tight (e.g., 2).
	// This allows comparing 2 tabs, but opening a 3rd will close the oldest.
	const MaxPprofSessions = 2
	if len(pprofSessions) >= MaxPprofSessions {
		var oldestKey string
		var oldestTime time.Time
		first := true

		for k, s := range pprofSessions {
			if first || s.LastAccess.Before(oldestTime) {
				oldestTime = s.LastAccess
				oldestKey = k
				first = false
			}
		}

		if oldestKey != "" {
			s := pprofSessions[oldestKey]
			delete(pprofSessions, oldestKey) // Remove from map first
			if s.Cmd.Process != nil {
				s.Cmd.Process.Kill()
			}
			logger.InfoGlobal().Msgf("Evicted oldest pprof session: %s", oldestKey)
		}
	}

	// 4. Start new
	port, err := getFreePort()
	if err != nil {
		return 0, err
	}

	var cmd *exec.Cmd
	// Detect file type to choose the right tool
	if strings.HasSuffix(targetPath, "trace.out") {
		// go tool trace -http=localhost:PORT {file}
		// Note: trace command might try to open a browser window on local dev environment.
		// We use -http flag to bind to specific port.
		cmd = exec.Command("go", "tool", "trace", fmt.Sprintf("-http=localhost:%d", port), targetPath)
	} else {
		// go tool pprof -http=localhost:PORT -no_browser {file}
		cmd = exec.Command("go", "tool", "pprof", fmt.Sprintf("-http=localhost:%d", port), "-no_browser", targetPath)
	}

	// We should probably set PPROF_TMPDIR or something if needed, but default is fine.

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start tool: %v", err)
	}

	// Give it a moment to start listening (trace parsing can be slow)
	time.Sleep(1500 * time.Millisecond)

	pprofSessions[targetPath] = &PprofSession{
		Port:       port,
		Cmd:        cmd,
		LastAccess: time.Now(),
	}

	go func() {
		cmd.Wait()
		pprofMutex.Lock()
		// Only delete if it is still the same session (check pointer or cmd)
		if s, ok := pprofSessions[targetPath]; ok && s.Cmd == cmd {
			delete(pprofSessions, targetPath)
		}
		pprofMutex.Unlock()
	}()

	return port, nil
}

func startPprofCleanupMonitor() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			pprofMutex.Lock()
			now := time.Now()
			for k, s := range pprofSessions {
				// Cleanup dead
				if s.Cmd.ProcessState != nil && s.Cmd.ProcessState.Exited() {
					delete(pprofSessions, k)
					continue
				}
				// Cleanup Idle (> 15 mins)
				if now.Sub(s.LastAccess) > 15*time.Minute {
					delete(pprofSessions, k) // Remove first
					if s.Cmd.Process != nil {
						s.Cmd.Process.Kill()
					}
					logger.InfoGlobal().Msgf("Cleaned up idle pprof session: %s", k)
				}
			}
			pprofMutex.Unlock()
		}
	}()
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func handlePprofProxy(c *gin.Context) {
	// URL: /api/performance/ui/:session/*path
	// session ID is essentially the relative path to the file,
	// but we encoded it to be URL safe. Or just pass "timestamp__service__instance/filename" directly?
	// Gin :session will match until next slash.
	// But our "ID" contains slashes (folder/file).
	// So we should expect encoded session ID.
	// Let's assume frontend calls: /api/performance/ui?file={path}&path={path_in_ui}
	// Or define route as /api/performance/ui/*filepath
	// But pprof UI makes requests to root relative paths e.g. /ui/flamegraph.

	// Better strategy:
	// Frontend opens: /api/performance/ui/view?file={path} in new tab.
	// This handler renders a page that REWRITES links? No.

	// Best Strategy:
	// We need a path prefix proxy.
	// /api/performance/ui/<safe_file_key>/...
	// <safe_file_key> -> base64(path)

	safeKey := c.Param("key")
	proxyPath := c.Param("path")

	decodedBytes, err := base64.RawURLEncoding.DecodeString(safeKey)
	if err != nil {
		c.String(400, "Invalid key")
		return
	}
	relPath := string(decodedBytes)

	// Security:
	absStorage, _ := filepath.Abs("./storage/pprof")
	targetPath := filepath.Join(absStorage, relPath)
	if !strings.HasPrefix(targetPath, absStorage) {
		c.String(403, "Access denied")
		return
	}

	port, err := GetOrStartPprofSession(targetPath)
	if err != nil {
		c.String(500, "Failed to start pprof: "+err.Error())
		return
	}

	// Reverse Proxy
	director := func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = fmt.Sprintf("localhost:%d", port)
		// Strip the prefix (/api/performance/ui/<key>)
		// proxyPath already has the rest
		req.URL.Path = proxyPath

		// pprof UI assets sometimes refer to root.
		// E.g. <script src="/ui/jquery.js">.
		// If we serve under /api/.../ui/<key>/, the browser will request /api/.../ui/<key>/ui/jquery.js
		// So proxyPath will be /ui/jquery.js. This maps correctly to localhost:port/ui/jquery.js.
		// PERFECT.
	}

	proxy := &httputil.ReverseProxy{
		Director: director,
		ModifyResponse: func(r *http.Response) error {
			// Fix redirects from pprof (which assumes it is at root)
			if loc := r.Header.Get("Location"); loc != "" {
				// If pprof redirects to absolute path (e.g. "/ui"), rewrite it to be under our proxy path
				if strings.HasPrefix(loc, "/") {
					r.Header.Set("Location", "/api/performance/ui/"+safeKey+loc)
				}
			}
			return nil
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func handleDeletePerformanceHistory(c *gin.Context) {
	var body struct {
		Folders []string `json:"folders"`
	}

	if err := c.BindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	baseDir, _ := filepath.Abs("./storage/pprof")
	deletedCount := 0

	for _, folder := range body.Folders {
		// Security Check: simple basename check
		if strings.Contains(folder, "..") || strings.Contains(folder, "/") || strings.Contains(folder, "\\") {
			continue
		}

		targetPath := filepath.Join(baseDir, folder)
		// Double check prefix
		if !strings.HasPrefix(targetPath, baseDir) {
			continue
		}

		if err := os.RemoveAll(targetPath); err == nil {
			deletedCount++

			// Also clean up any active pprof session
			pprofMutex.Lock()
			// Sessions keys are "folder/file". We need to find keys starting with folder.
			for k, s := range pprofSessions {
				if strings.HasPrefix(k, folder+"/") || strings.HasPrefix(k, folder+"\\") {
					if s.Cmd.Process != nil {
						s.Cmd.Process.Kill()
					}
					delete(pprofSessions, k)
				}
			}
			pprofMutex.Unlock()
		}
	}

	c.JSON(200, gin.H{"success": true, "deleted": deletedCount})
}

// ... initRegistry ...
func initRegistry() {
	methodRegistry = make(map[string]GenericHandler)

	methodRegistry["ValidateToken"] = func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error) {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid json: %v", err)
		}
		// BaseClient is embedded in CGClient, so we can call its methods directly
		uid, username, expiry, err := p.CGClient.ValidateToken(ctx, req.Token)
		if err != nil {
			return nil, err
		}
		return gin.H{"valid": true, "user_id": uid, "username": username, "expires_at": expiry}, nil
	}

	methodRegistry["GetBalance"] = func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error) {
		var req struct {
			UserID int64 `json:"user_id"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		bal, err := p.CGClient.GetBalance(ctx, req.UserID)
		if err != nil {
			return nil, err
		}
		return gin.H{"balance": bal}, nil
	}

	methodRegistry["AddBalance"] = func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error) {
		var req struct {
			UserID int64  `json:"user_id"`
			Amount int64  `json:"amount"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		newBal, err := p.CGClient.AddBalance(ctx, req.UserID, req.Amount, req.Reason)
		if err != nil {
			return nil, err
		}
		return gin.H{"new_balance": newBal}, nil
	}

	methodRegistry["GetCurrentRound"] = func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error) {
		if p.CGClient == nil {
			return nil, fmt.Errorf("ColorGame client not available in project %s", p.Config.Name)
		}
		var req pbColorGame.ColorGameGetCurrentRoundReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		return p.CGClient.GetCurrentRound(ctx, &req)
	}

	methodRegistry["GetState"] = func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error) {
		if p.CGClient == nil {
			return nil, fmt.Errorf("ColorGame client not available in project %s", p.Config.Name)
		}
		var req pbColorGame.ColorGameGetStateReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		return p.CGClient.GetState(ctx, &req)
	}

	methodRegistry["TestBroadcast"] = func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error) {
		if p.CGClient == nil {
			return nil, fmt.Errorf("ColorGame client not available in project %s", p.Config.Name)
		}
		var req struct {
			GameCode string `json:"game_code"`
			RoundID  string `json:"round_id"`
			State    string `json:"state"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}

		// Create a test broadcast message
		brc := &pbColorGame.ColorGameRoundStateBRC{
			RoundId: req.RoundID,
			State:   pbColorGame.ColorGameState_GAME_STATE_BETTING,
		}

		// Call Broadcast (BaseClient is embedded in CGClient)
		p.CGClient.Broadcast(ctx, req.GameCode, brc)

		return gin.H{
			"success":   true,
			"message":   "Broadcast sent",
			"game_code": req.GameCode,
			"round_id":  req.RoundID,
		}, nil
	}
}

func mustSub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		logger.WarnGlobal().Err(err).Msgf("Failed to sub fs %s", dir)
		return f
	}
	return sub
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
