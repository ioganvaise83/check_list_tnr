# Frontend Dockerfile
FROM nginx:alpine

# Копируем конфигурацию nginx
COPY nginx.conf /etc/nginx/nginx.conf

# Копируем статические файлы
COPY checklist_tnr_v2.html /usr/share/nginx/html/index.html

# Создаем директории для логов
RUN mkdir -p /var/log/nginx /var/cache/nginx

# Даем права на запись для логов
RUN touch /var/log/nginx/access.log /var/log/nginx/error.log && \
    chmod 644 /var/log/nginx/access.log /var/log/nginx/error.log

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
