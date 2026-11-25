FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl openssh-client

COPY --chmod=0555 entrypoint.sh /usr/local/bin/telegram-proxy-tunnel
COPY --chmod=0555 healthcheck.sh /usr/local/bin/telegram-proxy-check

ENTRYPOINT ["/usr/local/bin/telegram-proxy-tunnel"]
