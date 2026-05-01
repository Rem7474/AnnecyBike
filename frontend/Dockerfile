FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
# SPA fallback: toutes les routes → index.html
RUN printf 'server{\n  listen 80;\n  root /usr/share/nginx/html;\n  index index.html;\n  location / { try_files $uri $uri/ /index.html; }\n  location ~* \\.(js|css|png|svg|ico|woff2?)$ { expires 1y; add_header Cache-Control "public,immutable"; }\n}\n' > /etc/nginx/conf.d/default.conf
EXPOSE 80
