# Build the Vue application once, then serve only its static output.
FROM node:22.16.0-alpine AS frontend-builder

WORKDIR /frontend

COPY vue/package.json vue/package-lock.json ./
RUN npm ci

COPY vue ./
RUN npm run build

FROM nginx:1.27.5-alpine

COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=frontend-builder /frontend/dist /usr/share/nginx/html

EXPOSE 80
