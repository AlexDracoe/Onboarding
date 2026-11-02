# Shared: Node plus installed dependencies (cached until package*.json changes)
FROM node:26-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# Dev: Vite dev server with hot reload (used by `docker compose --profile dev watch`)
FROM deps AS dev
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev"]

# Build the static files
FROM deps AS build
COPY . .
RUN npm run build

# Serve them (default target: the last stage)
FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
