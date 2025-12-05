# Project Status

## Current Phase: Monolith Refinement

**Status:** Active
**Last Updated:** 2025-12-05

### Directives
- **Focus:** Perfecting the monolith architecture.
- **Constraint:** Do NOT modify microservices code (e.g., `cmd/color_game/microservices/...`) unless absolutely necessary or explicitly requested. This is to prevent unnecessary changes and potential regressions in the microservices setup while the core logic is being refined in the monolith.
- **Goal:** Ensure all business logic, broadcasting, and settlement flows work perfectly in the monolith deployment before moving back to microservices.

### Recent Changes
- Refined `Broadcaster` interfaces to include `gameCode`.
- Updated `RoundUseCase` and `PlayerUseCase` to use `gameCode`.
- Updated `GatewayUseCase` to return `game_code` in JSON responses.
- Added duration logging to `SettleRound`.
- Removed `SETTLEMENT_COMPLETE` broadcast from `SettleRound`.

### Key Modules (Monolith)
- **User Module:** Handles authentication (JWT), registration, and session management. Critical for Gateway to verify requests.
- **Gateway Module:** Manages WebSocket connections, HTTP routing, and forwards requests to internal modules.
- **GMS (Game Machine Service):** Core game logic state machine (Betting -> Drawing -> Result).
- **GS (Game Service):** Player-centric logic (PlaceBet, SettleRound), interacts with Wallet.
- **Wallet Module:** Manages user balances (currently Mock service in monolith).
