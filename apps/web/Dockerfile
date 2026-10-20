FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web/package.json ./apps/web/package.json
RUN npm ci
COPY apps/web ./apps/web
RUN npm run build

FROM nginx:stable-alpine
COPY infra/web-nginx.conf /etc/nginx/nginx.conf
COPY --from=build /src/apps/web/dist /usr/share/nginx/html
COPY licenses/EasyUI-MIT.txt /usr/share/nginx/html/THIRD-PARTY-NOTICES.txt
USER nginx
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
