package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/frankieli/game_product/internal/modules/gateway/usecase"
	"github.com/frankieli/game_product/internal/modules/gateway/ws"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/frankieli/game_product/pkg/service"
	"github.com/gorilla/websocket"
)

// Handler handles HTTP/WebSocket requests
type Handler struct {
	useCase *usecase.GatewayUseCase
	manager *ws.Manager
	userSvc service.UserService
}

// NewHandler creates a new HTTP handler
func NewHandler(useCase *usecase.GatewayUseCase, manager *ws.Manager, userSvc service.UserService) *Handler {
	return &Handler{
		useCase: useCase,
		manager: manager,
		userSvc: userSvc,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// HandleWebSocket handles websocket requests
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Create context with Request ID for WebSocket
	ctx := logger.WebSocketContext(r)
	requestID := logger.GetRequestID(ctx)

	logger.Info(ctx).
		Str("remote_addr", r.RemoteAddr).
		Msg("WebSocket 连接请求")

	// 1. Extract token from query param or header
	token := r.URL.Query().Get("token")
	if token == "" {
		logger.Warn(ctx).Msg("缺少认证 token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Validate token
	userID, _, _, err := h.userSvc.ValidateToken(r.Context(), token)
	if err != nil {
		logger.Warn(ctx).
			Err(err).
			Msg("Token 验证失败")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logger.Info(ctx).
		Int64("user_id", userID).
		Msg("Token 验证成功")

	// 3. Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error(ctx).Err(err).Msg("WebSocket 升级失败")
		return
	}

	logger.Info(ctx).
		Int64("user_id", userID).
		Msg("WebSocket 连接建立成功")

	// 4. Register client
	client := h.manager.Register(conn, userID)

	// Start pumps
	go client.WritePump()
	go client.ReadPump(func(userID int64, message []byte) {
		// Create new context with Request ID for each message
		msgCtx := logger.WithRequestID(context.Background(), logger.GenerateRequestID())
		msgCtx = logger.WithFields(msgCtx, map[string]interface{}{
			"user_id":       userID,
			"ws_request_id": requestID, // Original WS connection ID
		})

		logger.Debug(msgCtx).
			Int("message_size", len(message)).
			Msg("收到 WebSocket 消息")

		// Forward to Game Service via UseCase
		response, err := h.useCase.HandleMessage(msgCtx, userID, message)
		if err != nil {
			logger.Error(msgCtx).
				Err(err).
				Msg("处理消息失败")

			// Send error response to client
			errorResp := map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			}
			if jsonResp, err := json.Marshal(errorResp); err == nil {
				h.manager.SendToUser(userID, jsonResp)
			}
		} else if response != nil {
			h.manager.SendToUser(userID, response)
			logger.Debug(msgCtx).
				Int("response_size", len(response)).
				Msg("发送响应成功")
		}
	})
}
