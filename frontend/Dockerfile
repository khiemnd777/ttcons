FROM node:22-alpine AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY index.html postcss.config.js tailwind.config.js tsconfig.json tsconfig.node.json vite.config.ts ./
COPY public ./public
COPY src ./src

ARG VITE_API_URL=/api
ENV VITE_API_URL=${VITE_API_URL}

RUN npm run build

FROM caddy:2.11.3-alpine

COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /usr/share/caddy

RUN TRUSTED_PROXY_CIDR=127.0.0.1/32 REAL_IP_HEADER=X-Forwarded-For \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/gateway-health || exit 1
