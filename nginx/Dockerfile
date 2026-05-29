FROM node:20-alpine AS frontend-build

WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN apk add --no-cache imagemagick \
    && npm run build \
    && magick build/icon-512.png -resize 180x180 build/apple-touch-icon.png \
    && magick build/icon-512.png -resize 192x192 build/icon-192.png

FROM openresty/openresty:bullseye

RUN apt-get update && apt-get install -y \
    curl \
    git \
    luarocks \
    && rm -rf /var/lib/apt/lists/*

RUN luarocks install lua-resty-http
RUN luarocks install lua-resty-openssl
RUN luarocks install lua-resty-uuid

COPY --from=frontend-build /app/build /usr/share/nginx/html
COPY nginx/lua/ /etc/nginx/lua/
COPY nginx/nginx.conf /usr/local/openresty/nginx/conf/nginx.conf

EXPOSE 80
CMD ["openresty", "-g", "daemon off;"]
