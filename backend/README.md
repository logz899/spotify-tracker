# Spotify Backend - Go/Gin Refactor

This backend has been refactored from Python/FastAPI to Go/Gin for improved performance, type safety, and scalability.

## Tech Stack

- **Go 1.26** - Programming language
- **Gin** - HTTP web framework
- **pgx/v5** - PostgreSQL driver
- **zmb3/spotify/v2** - Spotify Web API client
- **PostgreSQL 13** - Database
- **Docker** - Containerization

## Project Structure

```
backend/
├── cmd/
│   └── api/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── database/
│   │   └── database.go          # Database operations
│   ├── handlers/
│   │   └── handlers.go          # HTTP handlers
│   ├── models/
│   │   └── song.go              # Data models
│   └── spotify/
│       └── client.go            # Spotify API client
├── .env.example                 # Environment variables template
├── .gitignore                   # Git ignore file
├── Dockerfile                   # Docker configuration
├── go.mod                       # Go module file
├── go.sum                       # Go dependencies checksums
└── README.md                    # This file
```

## API Endpoints

### Health Check
- `GET /api/v1/health` - Check if the API is running

### Authentication
- `GET /api/v1/auth/login` - Start Spotify OAuth flow
- `GET /api/v1/auth/callback` - OAuth callback handler

### Songs
- `GET /api/v1/songs/recent` - Fetch recently played songs from Spotify and save to DB
- `GET /api/v1/songs/all` - Retrieve all songs from database
- `GET /` - Legacy endpoint (same as `/api/v1/songs/recent`)

## Setup and Installation

### Prerequisites
- Go 1.26 or higher
- Docker and Docker Compose
- Spotify Developer Account (for Client ID)

### Local Development

1. Install dependencies:
```bash
go mod download
```

2. Create `.env` file (copy from `.env.example`):
```bash
cp .env.example .env
```

3. Update `.env` with your Spotify Client ID

4. Run locally:
```bash
go run cmd/api/main.go
```

### Docker Development

1. Build and run with Docker Compose:
```bash
cd ..  # Go to project root
docker-compose up --build
```

2. The backend will be available at `http://localhost:8000`

## Usage Flow

1. **Authenticate with Spotify:**
   ```bash
   curl http://localhost:8000/api/v1/auth/login
   ```
   Follow the redirect to authorize the application.

2. **Get Recently Played Songs:**
   ```bash
   curl http://localhost:8000/api/v1/songs/recent
   ```
   This will fetch your last 50 recently played tracks from Spotify, aggregate them by play count, and save to the database.

3. **Get All Songs from Database:**
   ```bash
   curl http://localhost:8000/api/v1/songs/all
   ```

## Environment Variables

Configuration is validated at startup by `internal/config`; production refuses
to start with missing or unsafe values. See `.env.example` for local values.

| Variable | Description | Production source |
|----------|-------------|-------------------|
| `APP_ENV` | `development`, `test`, or `production` | Env var |
| `GIN_MODE` | Must be `release` in production | Env var |
| `SERVER_PORT` | HTTP port (default `8000`) | Env var |
| `FRONTEND_URL` | GitHub Pages URL used for OAuth redirects | Env var |
| `CORS_ALLOWED_ORIGINS` | Comma-separated explicit origins; no wildcards | Env var |
| `SPOTIFY_CLIENT_ID` | Spotify application client ID | Env var |
| `SPOTIFY_REDIRECT_URI` | Cloud Run URL + `/api/v1/auth/spotify/callback` | Env var |
| `JWT_ISSUER`, `JWT_AUDIENCE` | JWT claims checked on every request | Env var |
| `SPOTIFY_CLIENT_SECRET` | Spotify application client secret | Secret Manager `spotify-client-secret` |
| `JWT_SECRET` | At least 32 bytes | Secret Manager `jwt-secret` |
| `DATABASE_URL` | PostgreSQL URL; production requires `sslmode=require` or stricter | Secret Manager `database-url` |
| `TOKEN_ENCRYPTION_KEY` | Base64 of exactly 32 bytes (AES-256-GCM) | Secret Manager `token-encryption-key` |

## Commands

The image contains three binaries:

| Command | Purpose |
|---------|---------|
| `/app/server` | HTTP API (Cloud Run service) |
| `/app/migrate up` | Apply migrations from `DATABASE_URL`, then exit |
| `/app/sync` | Synchronize every linked user once, then exit |

## Deployment

Production runs on Cloud Run with a Neon Free PostgreSQL database; the frontend
is a static GitHub Pages site. Definitions, one-time setup commands, and cost
controls are in [`deploy/cloudrun/README.md`](../deploy/cloudrun/README.md).

## Improvements Over Python Version

1. **Performance:** Go's compiled nature and efficient concurrency make it faster
2. **Type Safety:** Static typing catches errors at compile time
3. **Dependencies:** Single binary deployment, no Python runtime needed
4. **Memory:** Lower memory footprint compared to Python
5. **Structure:** Better organized code with clear separation of concerns
6. **Error Handling:** Explicit error handling throughout the codebase
7. **Graceful Shutdown:** Proper signal handling for clean shutdowns
8. **CORS Configuration:** More secure and configurable CORS setup
9. **Connection Pooling:** Efficient database connection management with pgx
10. **API Versioning:** Proper API versioning structure (`/api/v1/...`)

## Migration Notes

### Changes from Python Backend

- OAuth flow now requires explicit authentication via `/api/v1/auth/login` endpoint
- Spotify OAuth state and tokens are persisted in PostgreSQL.
- CORS configuration is more restrictive (configurable in main.go)
- Database operations use connection pooling
- All environment variables are properly typed and validated

### TODO for Production

- [x] Persist OAuth state and encrypted Spotify tokens in PostgreSQL
- [x] Add JWT authentication for API endpoints
- [ ] Implement rate limiting
- [ ] Add comprehensive logging (structured logging)
- [ ] Add metrics and monitoring (Prometheus)
- [ ] Implement request tracing
- [ ] Add comprehensive unit and integration tests
- [ ] Set up CI/CD pipeline
- [x] Move secrets to Google Secret Manager
- [x] Add database migrations tool (golang-migrate)

## Testing

Run tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

## Building

Build the binary:
```bash
go build -o server ./cmd/api
```

Run the binary:
```bash
./server
```

## License

[Your License Here]
