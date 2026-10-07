# ── Stage 1: build the React bundle ──────────────────────────────────────────
FROM node:18-alpine AS builder

WORKDIR /app

COPY package*.json ./
# Build tooling (react-scripts) is a devDependency, so install everything
RUN npm ci --no-audit --no-fund

COPY . .

# REACT_APP_API_URL is a CRA build-time variable baked into the JS bundle.
# Pass it at build time:
#   docker build --build-arg REACT_APP_API_URL=https://your-domain.com .
ARG REACT_APP_API_URL
ENV REACT_APP_API_URL=${REACT_APP_API_URL}

RUN npm run build

# ── Stage 2: serve with nginx ─────────────────────────────────────────────────
FROM nginx:stable-alpine

COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]
