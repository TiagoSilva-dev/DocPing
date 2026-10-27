FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web/package.json ./apps/web/package.json
RUN npm ci
COPY apps/web ./apps/web
# Vite bakes these into the bundle, so they are build arguments and not run
# time variables: changing one requires rebuilding the image. All of them are
# public by nature -- a measurement id travels in the page -- and all of them
# are optional. Absent means the tag is never loaded and the cookie notice
# never appears, which is the correct behaviour for a build with no account
# behind it. See apps/web/.env.example.
ARG VITE_GA4_ID=""
ARG VITE_GOOGLE_ADS_ID=""
ARG VITE_ADS_SIGNUP_LABEL=""
ARG VITE_MEETING_URL=""
ARG VITE_COMPANY_LEGAL_NAME=""
ARG VITE_COMPANY_TAX_ID=""
ARG VITE_COMPANY_ADDRESS=""
ENV VITE_GA4_ID=$VITE_GA4_ID \
    VITE_GOOGLE_ADS_ID=$VITE_GOOGLE_ADS_ID \
    VITE_ADS_SIGNUP_LABEL=$VITE_ADS_SIGNUP_LABEL \
    VITE_MEETING_URL=$VITE_MEETING_URL \
    VITE_COMPANY_LEGAL_NAME=$VITE_COMPANY_LEGAL_NAME \
    VITE_COMPANY_TAX_ID=$VITE_COMPANY_TAX_ID \
    VITE_COMPANY_ADDRESS=$VITE_COMPANY_ADDRESS
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
