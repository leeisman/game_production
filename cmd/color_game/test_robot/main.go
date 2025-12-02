package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/frankieli/game_product/pkg/logger"
	"github.com/gorilla/websocket"
)

// Config holds the robot configuration
type Config struct {
	Host      string
	UserCount int
	BetMin    int
	BetMax    int
}

// Robot represents a simulated player
type Robot struct {
	ID       int
	Host     string
	Username string
	Password string
	Token    string
	UserID   int64
	Conn     *websocket.Conn
	Done     chan struct{}
	ctx      context.Context
}

// API Responses
type RegisterResponse struct {
	UserID  int64  `json:"user_id"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type LoginResponse struct {
	UserID int64  `json:"user_id"`
	Token  string `json:"token"`
	Error  string `json:"error"`
}

type GameEvent struct {
	Type    string          `json:"type"`
	RoundID string          `json:"round_id"`
	Data    json.RawMessage `json:"data"`
}

func main() {
	// Parse command line arguments
	host := flag.String("host", "localhost:8081", "Server host address")
	users := flag.Int("users", 3010, "Number of concurrent users")
	flag.Parse()

	config := Config{
		Host:      *host,
		UserCount: *users,
		BetMin:    10,
		BetMax:    100,
	}

	logger.Init(logger.Config{
		Level:  "info",
		Format: "console",
	})

	ctx := context.Background()
	logger.Info(ctx).
		Int("users", config.UserCount).
		Str("host", config.Host).
		Msg("🤖 Starting Test Robot")

	var wg sync.WaitGroup
	wg.Add(config.UserCount)

	// Create and start robots
	for i := 0; i < config.UserCount; i++ {
		time.Sleep(20 * time.Millisecond)
		go func(id int) {
			defer wg.Done()
			robot := NewRobot(id, config.Host)
			if err := robot.Run(); err != nil {
				logger.Error(ctx).Int("robot_id", id).Err(err).Msg("Robot failed")
			}
		}(i + 1)
	}

	// Wait for interrupt signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt

	logger.Info(ctx).Msg("🛑 Stopping robots...")
}

func NewRobot(id int, host string) *Robot {
	return &Robot{
		ID:       id,
		Host:     host,
		Username: fmt.Sprintf("robot_%d_%d", time.Now().Unix(), id), // Unique username
		Password: "password123",
		Done:     make(chan struct{}),
		ctx:      context.Background(),
	}
}

func (r *Robot) Run() error {
	// 1. Register
	if err := r.Register(); err != nil {
		return fmt.Errorf("register failed: %w", err)
	}
	logger.Info(r.ctx).Int("robot_id", r.ID).Str("username", r.Username).Int64("user_id", r.UserID).Msg("Robot registered")

	// 2. Login
	if err := r.Login(); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	logger.Info(r.ctx).Int("robot_id", r.ID).Msg("Robot logged in")

	// 3. Connect WebSocket
	if err := r.ConnectWS(); err != nil {
		return fmt.Errorf("websocket connect failed: %w", err)
	}
	defer r.Conn.Close()
	logger.Info(r.ctx).Int("robot_id", r.ID).Msg("Robot connected to WebSocket")

	// 4. Listen and Play
	go r.ListenLoop()

	// Keep running until done
	<-r.Done
	return nil
}

func (r *Robot) Register() error {
	var err error
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(time.Second * time.Duration(i))
			logger.Info(r.ctx).Int("robot_id", r.ID).Int("retry", i).Msg("Retrying registration...")
		}

		url := fmt.Sprintf("http://%s/api/users/register", r.Host)
		body := map[string]string{
			"username": r.Username,
			"password": r.Password,
			"email":    fmt.Sprintf("%s@example.com", r.Username),
		}
		jsonBody, _ := json.Marshal(body)

		resp, reqErr := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if reqErr != nil {
			err = reqErr
			continue
		}
		defer resp.Body.Close()

		var result RegisterResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
			err = decodeErr
			continue
		}

		if !result.Success && result.Error != "" {
			err = fmt.Errorf(result.Error)
			// If user already exists, we can consider it a success (or handle login)
			// But for now, let's just retry or fail.
			// Actually, if user exists, we should probably just proceed to login.
			if result.Error == "username already exists" {
				logger.Info(r.ctx).Int("robot_id", r.ID).Msg("User already exists, proceeding to login")
				return nil
			}
			continue
		}

		r.UserID = result.UserID
		return nil
	}
	return fmt.Errorf("register failed after 3 retries: %w", err)
}

func (r *Robot) Login() error {
	var err error
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(time.Second * time.Duration(i))
			logger.Info(r.ctx).Int("robot_id", r.ID).Int("retry", i).Msg("Retrying login...")
		}

		url := fmt.Sprintf("http://%s/api/users/login", r.Host)
		body := map[string]string{
			"username": r.Username,
			"password": r.Password,
		}
		jsonBody, _ := json.Marshal(body)

		resp, reqErr := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if reqErr != nil {
			err = reqErr
			continue
		}
		defer resp.Body.Close()

		var result LoginResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
			err = decodeErr
			continue
		}

		if result.Token == "" {
			err = fmt.Errorf(result.Error)
			continue
		}

		r.Token = result.Token
		r.UserID = result.UserID // Update UserID just in case
		return nil
	}
	return fmt.Errorf("login failed after 3 retries: %w", err)
}

func (r *Robot) ConnectWS() error {
	u := url.URL{Scheme: "ws", Host: r.Host, Path: "/ws", RawQuery: "token=" + r.Token}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	r.Conn = c
	return nil
}

func (r *Robot) CheckState() {
	req := map[string]interface{}{
		"game":    "color_game",
		"command": "get_state",
	}
	if err := r.Conn.WriteJSON(req); err != nil {
		logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Failed to check state")
	}
}

func (r *Robot) ListenLoop() {
	defer close(r.Done)

	for {
		_, message, err := r.Conn.ReadMessage()
		if err != nil {
			logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Read error")
			return
		}

		var event GameEvent
		if err := json.Unmarshal(message, &event); err != nil {
			logger.Warn(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Failed to parse message")
			continue
		}

		switch event.Type {
		case "betting_started":
			go r.PlaceBet(event.RoundID)
		case "game_state":
			var data struct {
				RoundID string `json:"round_id"`
				State   string `json:"state"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				logger.Warn(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Failed to parse game_state data")
				continue
			}
			logger.Info(r.ctx).Int("robot_id", r.ID).Str("round_id", data.RoundID).Str("state", data.State).Msg("Received game state")
			if data.State == "BETTING" {
				go r.PlaceBet(data.RoundID)
			}
		case "result":
			logger.Info(r.ctx).Int("robot_id", r.ID).Str("round_id", event.RoundID).Str("result", string(event.Data)).Msg("Saw result")
		case "settlement":
			logger.Info(r.ctx).Int("robot_id", r.ID).Str("data", string(event.Data)).Msg("Received settlement")
		}
	}
}

func (r *Robot) PlaceBet(roundID string) {
	// Random delay to simulate human behavior
	// Spread out bets over 5 seconds to avoid DB spikes causing heartbeat timeouts
	time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)

	colors := []string{"red", "green", "blue"}
	color := colors[rand.Intn(len(colors))]
	amount := (rand.Intn(10) + 1) * 10 // 10, 20, ..., 100

	req := map[string]interface{}{
		"game":    "color_game",
		"command": "place_bet",
		"color":   color,
		"amount":  amount,
	}

	if err := r.Conn.WriteJSON(req); err != nil {
		logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Failed to place bet")
		return
	}

	logger.Info(r.ctx).Int("robot_id", r.ID).Int("amount", amount).Str("color", color).Str("round_id", roundID).Msg("Placed bet")
}
