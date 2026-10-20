FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web/package.json ./apps/web/package.json
RUN npm ci
COPY apps/web ./apps/web
RUN npm run build

FROM nginx:stable-alpine
# The base image's entrypoint renders /etc/nginx/templates/*.template with
# envsubst before starting, which is how PORT and the API address reach the
# config: Railway assigns PORT at run time and nginx cannot read it itself.
COPY infra/web-nginx.conf.template /etc/nginx/templates/nginx.conf.template
COPY --from=build /src/apps/web/dist /usr/share/nginx/html
COPY licenses/EasyUI-MIT.txt /usr/share/nginx/html/THIRD-PARTY-NOTICES.txt
ENV NGINX_ENVSUBST_OUTPUT_DIR=/etc/nginx \
    NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1 \
    PORT=8080 \
    API_UPSTREAM=http://api:8080
RUN chown -R nginx:nginx /etc/nginx
USER nginx
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
