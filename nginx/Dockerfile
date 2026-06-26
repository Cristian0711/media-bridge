FROM node:26-alpine AS frontend-build

WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN apk add --no-cache imagemagick \
    && npm run build \
    # Pad-to-exact-square so icons always match the sizes declared in the manifest,
    # even if the source image isn't square (e.g. a 3:2 logo). Derive the smaller
    # icons from the source first, then normalize icon-512 itself last.
    && magick build/icon-512.png -resize 192x192 -background "#0b0b0f" -gravity center -extent 192x192 build/icon-192.png \
    && magick build/icon-512.png -resize 180x180 -background "#0b0b0f" -gravity center -extent 180x180 build/apple-touch-icon.png \
    && magick build/icon-512.png -resize 512x512 -background "#0b0b0f" -gravity center -extent 512x512 build/icon-512.png

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
