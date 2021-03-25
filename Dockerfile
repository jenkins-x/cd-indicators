FROM alpine:3.13

RUN apk add --no-cache ca-certificates \
 && adduser -D -u 1000 jx

COPY ./build/linux/cd-indicators /app/

WORKDIR /app
USER 1000

ENTRYPOINT ["/app/cd-indicators"]