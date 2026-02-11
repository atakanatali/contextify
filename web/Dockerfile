FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

COPY . .
RUN npm run build

FROM nginx:alpine

LABEL org.opencontainers.image.title="Contextify Web UI" \
      org.opencontainers.image.description="Web dashboard for Contextify memory system" \
      org.opencontainers.image.source="https://github.com/atakanatali/contextify" \
      org.opencontainers.image.licenses="MIT"

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]
