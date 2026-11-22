# Custom API gateway image.
#
# The stable Alpine tag is used deliberately: dynamic upstream re-resolution
# (`resolve` in an upstream server) requires NGINX 1.27.3 or newer, and the
# stable tag satisfies that requirement. Pin an exact patch tag in production
# if reproducible image digests are required.

FROM nginx:stable-alpine

# Remove the stock default server and route relative log/pid paths to the
# image's stdout/stderr-backed log directory.
RUN rm -f /etc/nginx/conf.d/default.conf \
    && rm -rf /etc/nginx/logs \
    && ln -s /var/log/nginx /etc/nginx/logs

COPY nginx.conf /etc/nginx/nginx.conf
COPY conf.d/10-default.conf /etc/nginx/conf.d/10-default.conf
COPY compose/00-upstreams.conf /etc/nginx/conf.d/00-upstreams.conf
COPY snippets/ /etc/nginx/snippets/

# Fail the image build if the gateway configuration is invalid.
RUN nginx -t

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
