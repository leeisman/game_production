# Design Principles

## Module Communication
- **Internal Module Interfaces**: Interfaces defined in `domain` package are for internal module usage.
- **External Module Interfaces**: Interfaces for external module communication (e.g., Gateway, User Service) should be defined in `pkg/service`.
- **Gateway Communication**: Use `pkg/service.GatewayService` for broadcasting messages to the Gateway (and subsequently to users).

## Adapter Naming Convention
- **Unified Naming**: All adapter implementations should be named `Handler` (e.g., `local/handler.go`, `grpc/handler.go`) for consistency across the codebase.
- **Examples**:
  - `internal/modules/color_game/gs/adapter/local/handler.go`
  - `internal/modules/color_game/gms/adapter/local/handler.go`
  - `internal/modules/gateway/adapter/local/handler.go`
