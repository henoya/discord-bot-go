# Discord Bot Go - Developer Guide

## Build & Run Commands
- Build: `go build -o discord-bot-go`
- Run: `./discord-bot-go`
- Run with automatic restart: `./bot.sh`
- Test: `go test ./...`
- Test single file: `go test -v ./path/to/file_test.go`
- Lint: `golangci-lint run`
- Format code: `gofmt -w .`

## Code Style Guidelines
- Imports: Group standard library, third-party, and local imports with blank lines
- Error handling: Always check errors and propagate with context using named returns
- Logging: Use zap.SugaredLogger with named loggers for context
- Database: Use GORM for database operations
- Structure: Keep files focused on specific functionality (profile.go, bot.go, etc.)
- Naming: Use camelCase for variables, PascalCase for exported functions
- Environment: Required environment variables are defined in readEnvs function

## Project Structure
- Discord bot with Bluesky integration
- SQLite database for persistence
- Echo web framework for HTTP endpoints
- Configuration via environment variables