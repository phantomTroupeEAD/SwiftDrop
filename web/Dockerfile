# syntax=docker/dockerfile:1.7
# Stage 1: Build React/TypeScript Single Page Application
FROM node:20-alpine AS builder

WORKDIR /app

# Install dependencies first for efficient layer caching
COPY package.json package-lock.json ./
RUN npm ci

# Copy full frontend source code
COPY . .

# Set canonical base API URL (Agreement A3 & C15: /api/v1 for unified edge proxying)
ENV VITE_API_BASE_URL=/api/v1

# Compile TypeScript and build production bundle via Vite
RUN npm run build

# Stage 2: Production Nginx Server (Hardened, Non-root USER 10001)
FROM nginx:1.25-alpine

# Create unprivileged service user and group matching SwiftDrop standard (SEC-016)
RUN addgroup -g 10001 -S swiftdrop && \
    adduser -u 10001 -S swiftdrop -G swiftdrop && \
    mkdir -p /var/cache/nginx /var/run /var/log/nginx && \
    touch /var/run/nginx.pid && \
    chown -R swiftdrop:swiftdrop /var/cache/nginx /var/run /var/run/nginx.pid /var/log/nginx /usr/share/nginx/html

# Replace default Nginx server block with SwiftDrop SPA configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy compiled static assets from builder stage
COPY --from=builder --chown=swiftdrop:swiftdrop /app/dist /usr/share/nginx/html

# Switch to unprivileged non-root user
USER swiftdrop:swiftdrop

EXPOSE 3000

HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:3000/healthz || exit 1

CMD ["nginx", "-g", "daemon off;"]

