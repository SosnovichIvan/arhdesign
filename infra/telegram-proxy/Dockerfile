FROM alpine:3.22

RUN apk add --no-cache netcat-openbsd openssh-client

COPY entrypoint.sh /usr/local/bin/telegram-proxy-tunnel
RUN chmod 0555 /usr/local/bin/telegram-proxy-tunnel

ENTRYPOINT ["/usr/local/bin/telegram-proxy-tunnel"]
