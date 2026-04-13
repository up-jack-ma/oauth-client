.PHONY: dev build docker run clean

# Local development
dev-backend:
	cd backend && go run .

dev-frontend:
	cd frontend && npm run dev

# Build
build-frontend:
	cd frontend && npm ci && npm run build

build-backend:
	cd backend && CGO_ENABLED=1 go build -ldflags="-s -w" -o ../server .

build: build-frontend build-backend
	mkdir -p static && cp -r frontend/dist/* static/

# Docker
docker:
	docker build -t oauth-client .

docker-run:
	docker run -d --name oauth-client \
		-p 8080:8080 \
		-v oauth-data:/app/data \
		-e BASE_URL=http://localhost:8080 \
		-e JWT_SECRET=$$(openssl rand -hex 32) \
		oauth-client

# Docker Compose
up:
	docker compose up -d

up-with-nginx:
	docker compose --profile with-nginx up -d

down:
	docker compose down

logs:
	docker compose logs -f

# Clean
clean:
	rm -f server
	rm -rf frontend/dist static
