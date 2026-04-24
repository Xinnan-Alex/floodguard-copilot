FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
# Cloud Run expects the port to be 8080 by default, Nginx usually runs on 80.
# A custom nginx.conf can be used, or we just rely on exposing 80.
# Let's replace the default listen port to 8080 to comply seamlessly with Cloud Run defaults.
RUN sed -i 's/listen  *80;/listen 8080;/g' /etc/nginx/conf.d/default.conf
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
