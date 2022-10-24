FROM alpine:3.16

COPY --chown=65534 build/bin/linux_amd64/osprey /usr/local/bin/osprey
WORKDIR /

USER 65534
ENTRYPOINT ["osprey"]
