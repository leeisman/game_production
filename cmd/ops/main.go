package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
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
	NacosHost string `json:"-"`
	NacosPort string `json:"-"`
	Namespace string `json:"-"`
}

type ProjectContext struct {
	Config   ProjectConfig
	Registry discovery.Registry
	CGClient *color_game.Client // Contains BaseClient internally
}

var (
	projects    = make(map[string]*ProjectContext)
	projectList = []ProjectConfig{} // For frontend listing
)

// GenericHandler now accepts the project context
type GenericHandler func(ctx context.Context, p *ProjectContext, payload []byte) (interface{}, error)

var methodRegistry map[string]GenericHandler

func main() {
	logger.Init(logger.Config{
		Level:  "debug",
		Format: "console",
	})

	// 1. Initialize Multi-Project Configuration
	initProjects()

	// 2. Initialize Method Registry
	initRegistry()

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

	port := "7090"
	logger.InfoGlobal().Msgf("🚀 OPS Server running at http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to start OPS server")
	}
}

func initProjects() {
	// Define Game Projects here
	// Each game maps to a specific Nacos Namespace/Cluster
	configs := []ProjectConfig{
		{
			ID:        "color_game",
			Name:      "Color Game",
			NacosHost: "localhost",
			NacosPort: "8848",
			Namespace: "public", // Assuming Color Game uses public namespace for now
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
