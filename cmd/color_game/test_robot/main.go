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
	UserHost  string // User API host (for register/login)
	WSHost    string // WebSocket host (for game)
	UserCount int
	BetMin    int
	BetMax    int
}

// Robot represents a simulated player
type Robot struct {
	ID       int
	UserHost string // User API host
	WSHost   string // WebSocket host
	Username string
	Password string
	Token    string
	UserID   int64
	Conn     *websocket.Conn
	msgChan  chan []byte
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
	Game    string          `json:"game"`
	Command string          `json:"command"`
	Data    json.RawMessage `json:"data"`
}

func main() {
	// Parse command line arguments
	userHost := flag.String("user-host", "localhost:8082", "User API host address (for register/login)")
	wsHost := flag.String("ws-host", "localhost:8081", "WebSocket host address (for game)")
	users := flag.Int("users", 4510, "Number of concurrent users")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	config := Config{
		UserHost:  *userHost,
		WSHost:    *wsHost,
		UserCount: *users,
		BetMin:    10,
		BetMax:    100,
	}

	// Create logs directory if not exists
	// InitWithFile handles directory creation, rotation, and multi-output
	logger.InitWithFile("logs/color_game/robot.log", *logLevel, "console", true)
	defer logger.Flush()

	fmt.Printf("🤖 Starting Test Robot... Logs are being written to logs/color_game/test_robot.log (rotating)\n")

	ctx := context.Background()
	logger.Info(ctx).
		Int("users", config.UserCount).
		Str("user_host", config.UserHost).
		Str("ws_host", config.WSHost).
		Msg("🤖 Starting Test Robot")

	var wg sync.WaitGroup
	wg.Add(config.UserCount)

	// Create and start robots
	for i := 0; i < config.UserCount; i++ {
		time.Sleep(20 * time.Millisecond)
		go func(id int) {
			defer wg.Done()
			robot := NewRobot(id, config.UserHost, config.WSHost)
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

func NewRobot(id int, userHost, wsHost string) *Robot {
	return &Robot{
		ID:       id,
		UserHost: userHost,
		WSHost:   wsHost,
		Username: fmt.Sprintf("robot_user_%d", id), // Fixed username for reuse
		Password: "password123",
		ctx:      context.Background(),
	}
}

func (r *Robot) Run() error {
	// 1. Try Login first
	err := r.Login()
	if err == nil {
		logger.Info(r.ctx).Int("robot_id", r.ID).Msg("Login successful (existing user)")
	} else {
		// Login failed, try Register
		logger.Info(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Login failed, trying to register")

		if err := r.Register(); err != nil {
			return fmt.Errorf("register failed: %w", err)
		}
		logger.Info(r.ctx).Int("robot_id", r.ID).Str("username", r.Username).Int64("user_id", r.UserID).Msg("Robot registered")

		// Login again after register
		if err := r.Login(); err != nil {
			return fmt.Errorf("login after register failed: %w", err)
		}
		logger.Info(r.ctx).Int("robot_id", r.ID).Msg("Robot logged in after registration")
	}

	// 2. Connect WebSocket
	if err := r.ConnectWS(); err != nil {
		return fmt.Errorf("websocket connect failed: %w", err)
	}
	defer r.Conn.Close()
	logger.Info(r.ctx).Int("robot_id", r.ID).Msg("Robot connected to WebSocket")

	// 4. Start Read Pump
	r.msgChan = make(chan []byte, 10)
	go r.readPump()

	// 5. Event Loop
	betTimer := time.NewTimer(0)
	if !betTimer.Stop() {
		<-betTimer.C
	}
	var currentRoundID string

	for {
		select {
		case message, ok := <-r.msgChan:
			if !ok {
				return nil // Connection closed
			}
			r.handleMessage(message, &currentRoundID, betTimer)

		case <-betTimer.C:
			r.performBet(currentRoundID)

		case <-r.ctx.Done():
			return r.ctx.Err()
		}
	}
}

func (r *Robot) Register() error {
	var err error
	for i := 0; i < 20; i++ { // Retry 20 times
		if i > 0 {
			// Random sleep 100-300ms
			sleepMs := 100 + rand.Intn(500)
			time.Sleep(time.Duration(sleepMs) * time.Millisecond)
			logger.Warn(r.ctx).Int("robot_id", r.ID).Int("retry", i).Msg("Retrying registration...")
		}

		url := fmt.Sprintf("http://%s/api/users/register", r.UserHost)
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

		// Check for 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			err = fmt.Errorf("rate limited (429)")
			continue
		}

		// Check for non-200 status codes
		if resp.StatusCode != http.StatusOK {
			// Try to decode error response
			var errorResp struct {
				Error string `json:"error"`
			}
			if decodeErr := json.NewDecoder(resp.Body).Decode(&errorResp); decodeErr == nil {
				err = fmt.Errorf("%s", errorResp.Error)
				// If user already exists, proceed to login
				if errorResp.Error == "username already exists" {
					logger.Info(r.ctx).Int("robot_id", r.ID).Msg("User already exists, proceeding to login")
					return nil
				}
			} else {
				err = fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			continue
		}

		// Success case - decode RegisterResponse
		var result RegisterResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
			err = decodeErr
			continue
		}

		r.UserID = result.UserID
		return nil
	}
	// Log final failure as Error
	logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Register failed after 20 retries")
	return fmt.Errorf("register failed after 20 retries: %w", err)
}

func (r *Robot) Login() error {
	var err error
	for i := 0; i < 5; i++ { // Retry 5 times
		if i > 0 {
			// Random sleep 100-300ms
			sleepMs := 100 + rand.Intn(200)
			time.Sleep(time.Duration(sleepMs) * time.Millisecond)
			logger.Warn(r.ctx).Int("robot_id", r.ID).Int("retry", i).Msg("Retrying login...")
		}

		url := fmt.Sprintf("http://%s/api/users/login", r.UserHost)
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

		// Check for 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			err = fmt.Errorf("rate limited (429)")
			continue
		}

		// Check for non-200 status codes
		if resp.StatusCode != http.StatusOK {
			// Try to decode error response
			var errorResp struct {
				Error string `json:"error"`
			}
			if decodeErr := json.NewDecoder(resp.Body).Decode(&errorResp); decodeErr == nil {
				err = fmt.Errorf("%s", errorResp.Error)
			} else {
				err = fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			continue
		}

		// Success case - decode LoginResponse
		var result LoginResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
			err = decodeErr
			continue
		}

		r.Token = result.Token
		r.UserID = result.UserID // Update UserID just in case
		return nil
	}
	// Log final failure as Error
	logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Login failed after 5 retries")
	return fmt.Errorf("login failed after 5 retries: %w", err)
}

func (r *Robot) ConnectWS() error {
	u := url.URL{Scheme: "ws", Host: r.WSHost, Path: "/ws", RawQuery: "token=" + r.Token}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		// Log connection failure as Error
		logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("WebSocket connection failed")
		return err
	}
	r.Conn = c
	return nil
}

func (r *Robot) readPump() {
	defer close(r.msgChan)
	for {
		_, message, err := r.Conn.ReadMessage()
		if err != nil {
			logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Read error")
			return
		}
		r.msgChan <- message
	}
}

func (r *Robot) handleMessage(message []byte, currentRoundID *string, betTimer *time.Timer) {
	var event GameEvent
	if err := json.Unmarshal(message, &event); err != nil {
		logger.Warn(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Failed to parse message")
		return
	}

	switch event.Command {
	case "ColorGameRoundStateBRC":
		var data struct {
			RoundID string `json:"round_id"`
			State   string `json:"state"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Warn(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Failed to parse game_state data")
			return
		}
		logger.Info(r.ctx).Int("robot_id", r.ID).Str("round_id", data.RoundID).Str("state", data.State).Msg("Received game state")
		if data.State == "GAME_STATE_BETTING" {
			*currentRoundID = data.RoundID
			r.scheduleBet(betTimer)
		}

	case "ColorGameSettlementBRC":
		// logger.Info(r.ctx).Int("robot_id", r.ID).Msg("Received settlement")
	}
}

func (r *Robot) scheduleBet(timer *time.Timer) {
	// Random delay to simulate human behavior (0-3s)
	delay := time.Duration(rand.Intn(3000)) * time.Millisecond
	timer.Reset(delay)
}

func (r *Robot) performBet(roundID string) {
	colors := []string{"red", "green", "blue"}
	color := colors[rand.Intn(len(colors))]
	amount := (rand.Intn(10) + 1) * 10 // 10, 20, ..., 100

	req := map[string]interface{}{
		"game_code": "color_game",
		"command":   "ColorGamePlaceBetREQ",
		"data": map[string]interface{}{
			"color":  color,
			"amount": amount,
		},
	}

	if err := r.Conn.WriteJSON(req); err != nil {
		logger.Error(r.ctx).Int("robot_id", r.ID).Err(err).Msg("Failed to place bet")
		return
	}
}
