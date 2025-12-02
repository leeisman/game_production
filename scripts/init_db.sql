-- Create database schemas for Color Game Platform

-- ============================================
-- Auth Service Schema
-- ============================================

-- User accounts table for authentication and authorization
CREATE TABLE IF NOT EXISTS users (
    user_id BIGSERIAL PRIMARY KEY,                                    -- Unique user identifier (auto-increment)
    username VARCHAR(50) UNIQUE NOT NULL,                             -- Unique username for login (3-50 characters)
    password_hash VARCHAR(255) NOT NULL,                              -- Bcrypt hashed password
    email VARCHAR(100) UNIQUE NOT NULL,                               -- Unique email address for account recovery
    status INTEGER NOT NULL DEFAULT 1,                                -- Account status: 1=active, 2=suspended, 3=banned
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,          -- Account creation timestamp
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,          -- Account last update timestamp
    last_login_at TIMESTAMP                                           -- Last successful login timestamp
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

-- Active user sessions for JWT token management
CREATE TABLE IF NOT EXISTS sessions (
    session_id VARCHAR(64) PRIMARY KEY,                               -- Unique session identifier (UUID)
    user_id BIGINT NOT NULL,                                          -- Reference to users table
    token TEXT NOT NULL,                                              -- JWT token string
    expires_at TIMESTAMP NOT NULL,                                    -- Session expiration timestamp
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,          -- Session creation timestamp
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- ============================================
-- Color Game Service Schema
-- ============================================

-- Game round history and statistics for Color Game
-- Records each round from start to completion with betting statistics and results
CREATE TABLE IF NOT EXISTS game_rounds (
    round_id VARCHAR(64) PRIMARY KEY,                                 -- Unique round identifier (format: YYYYMMDDHHMMSS)
    game_code VARCHAR(32) NOT NULL,                                   -- Game type identifier (e.g., "color_game")
    status INTEGER NOT NULL DEFAULT 0,                                -- Round status: 0=in_progress (betting/drawing), 1=ended (result announced)
    start_time TIMESTAMP NOT NULL,                                    -- Round start timestamp (when round_started event fires)
    end_time TIMESTAMP,                                               -- Round end timestamp (when result event fires)
    result VARCHAR(512),                                              -- Game result (e.g., "red", "green", "blue", "yellow" for color game, or complex JSON for other games)
    total_bets INTEGER NOT NULL DEFAULT 0,                            -- Total number of bets placed during this round
    total_players INTEGER NOT NULL DEFAULT 0,                         -- Total number of unique players participated in this round
    total_bet_amount DECIMAL(18,2) NOT NULL DEFAULT 0,                -- Total amount wagered in this round (sum of all bets)
    created_at TIMESTAMP NOT NULL,                                    -- Record creation timestamp (application server time)
    updated_at TIMESTAMP NOT NULL                                     -- Record last update timestamp (application server time)
);

CREATE INDEX IF NOT EXISTS idx_game_rounds_game_code ON game_rounds(game_code);
CREATE INDEX IF NOT EXISTS idx_game_rounds_status ON game_rounds(status);
CREATE INDEX IF NOT EXISTS idx_game_rounds_start_time ON game_rounds(start_time);

-- Bet orders table for tracking individual player bets
-- Records each bet placed by players, stored in memory during betting and persisted to DB during settlement
CREATE TABLE IF NOT EXISTS bet_orders (
    order_id VARCHAR(64) PRIMARY KEY,                                 -- Unique bet order identifier (Snowflake ID)
    user_id BIGINT NOT NULL,                                          -- Player user ID
    round_id VARCHAR(64) NOT NULL,                                    -- Reference to game round
    game_code VARCHAR(32) NOT NULL,                                   -- Game type identifier (e.g., "color_game", "baccarat", "roulette")
    bet_area VARCHAR(512) NOT NULL,                                  -- Bet area/zone (e.g., "red" for color game, "player" for baccarat, "17" for roulette)
    amount DECIMAL(18,2) NOT NULL,                                    -- Bet amount
    payout DECIMAL(18,2) NOT NULL DEFAULT 0,                          -- Payout amount (0 for lose, amount * odds for win)
    status INTEGER NOT NULL DEFAULT 0,                                -- Bet status: 0=pending, 1=settled
    created_at TIMESTAMP NOT NULL,                                    -- Bet placement timestamp (application server time)
    settled_at TIMESTAMP                                              -- Settlement timestamp (application server time)
);

CREATE INDEX IF NOT EXISTS idx_bet_orders_user_id ON bet_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_bet_orders_round_id ON bet_orders(round_id);
CREATE INDEX IF NOT EXISTS idx_bet_orders_game_code ON bet_orders(game_code);
CREATE INDEX IF NOT EXISTS idx_bet_orders_status ON bet_orders(status);
CREATE INDEX IF NOT EXISTS idx_bet_orders_created_at ON bet_orders(created_at);

