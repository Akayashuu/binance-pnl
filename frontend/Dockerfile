# syntax=docker/dockerfile:1.6

# --- builder ---------------------------------------------------------------
FROM node:22-alpine AS builder

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@10 --activate

# Manifest first for better caching
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --no-frozen-lockfile

COPY . .

ARG PUBLIC_API_BASE_URL=http://localhost:8080
ENV PUBLIC_API_BASE_URL=$PUBLIC_API_BASE_URL

RUN pnpm build

# --- runtime ---------------------------------------------------------------
FROM nginx:alpine

COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
