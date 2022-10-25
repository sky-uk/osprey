FROM alpine:3.16

RUN apk add --update --no-cache libc6-compat
COPY --chown=65534 build/bin/linux_amd64/osprey /usr/local/bin/osprey
WORKDIR /

USER 65534
ENTRYPOINT ["osprey"]
