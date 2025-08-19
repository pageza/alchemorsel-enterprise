# Alchemorsel v3 Quick Start Guide

Get up and running with Alchemorsel v3 in under 5 minutes!

## 🚀 Prerequisites

- Docker 20.10+ and Docker Compose 2.0+
- Git
- 4GB+ RAM available

## ⚡ 30-Second Setup

```bash
# Clone the repository
git clone <repository-url>
cd alchemorsel-v3

# Setup environment
cp .env.example .env

# Start local development environment
./scripts/docker-dev.sh local up
```

## 🌐 Access Your Application

Once the containers are running:

- **Web Application:** http://localhost:8080
- **API Backend:** http://localhost:3000
- **API Docs:** http://localhost:3000/docs
- **Health Check:** http://localhost:3000/health

## 🔑 First Steps

### 1. Register a New Account
1. Go to http://localhost:8080
2. Click "Register" 
3. Fill in your details
4. You'll be automatically logged in

### 2. Create Your First Recipe
1. Navigate to "Recipes" 
2. Click "New Recipe"
3. Fill in recipe details
4. Save your recipe

### 3. Try AI Recipe Generation
1. Go to "AI Chef"
2. Describe what you want to cook
3. Let Claude generate a recipe for you
4. Save the generated recipe

## 🛠️ Development Workflow

### Local Development (No Docker)
```bash
# Start dependencies only
docker-compose -f docker-compose.local.yml up postgres redis -d

# Run API backend locally
PORT=3000 go run cmd/api-pure/main.go &

# Run Web frontend locally  
API_URL=http://localhost:3000 PORT=8080 go run cmd/web/main.go
```

### Run Tests
```bash
# Unit tests
go test ./...

# Integration tests
./scripts/docker-dev.sh local up
go test -tags=integration ./...
```

### View Logs
```bash
# All services
./scripts/docker-dev.sh local logs

# Specific service
docker-compose -f docker-compose.local.yml logs api-backend
```

## 🔧 Common Commands

```bash
# Stop everything
./scripts/docker-dev.sh local down

# Restart services
./scripts/docker-dev.sh local restart

# Check service status
./scripts/docker-dev.sh local status

# Rebuild services
./scripts/docker-dev.sh local build

# Open shell in API container
./scripts/docker-dev.sh local shell
```

## 🐛 Troubleshooting

### Services won't start?
```bash
# Check Docker
docker info

# Check logs
./scripts/docker-dev.sh local logs

# Clean restart
./scripts/docker-dev.sh local down
docker system prune -f
./scripts/docker-dev.sh local up
```

### Database issues?
```bash
# Reset database
docker-compose -f docker-compose.local.yml down -v
./scripts/docker-dev.sh local up
```

### Need to update your API key?
Edit `.env` file and update `ANTHROPIC_API_KEY=your-key-here`

## 📚 Next Steps

- [Full Docker Guide](./DOCKER.md)
- [API Documentation](./docs/api.md) 
- [Architecture Overview](./docs/architecture.md)
- [Deployment Guide](./docs/DEPLOYMENT.md)

## 🆘 Need Help?

- Check the logs: `./scripts/docker-dev.sh local logs`
- Review the [Docker Guide](./DOCKER.md)
- Open an issue on GitHub

Happy cooking! 👨‍🍳✨