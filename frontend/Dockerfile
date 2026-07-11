# --- Étape de build ---
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
# npm ci : install déterministe depuis le lock file.
# --no-audit/--no-fund : évite des appels réseau inutiles qui peuvent bloquer.
# Timeouts courts + retries : échoue vite au lieu de geler indéfiniment.
RUN npm config set fetch-retry-maxtimeout 30000 \
 && npm config set fetch-timeout 60000 \
 && npm ci --no-audit --no-fund --loglevel=info
COPY . .
RUN npm run build

# --- Serveur statique nginx ---
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
