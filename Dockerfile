FROM alpine:3.9

# OpenSSL is required so wget can query HTTPS endpoints for health checking.
RUN apk add --update ca-certificates openssl
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

COPY build/bin/linux_amd64/osprey /usr/local/bin/osprey
WORKDIR /

ENTRYPOINT ["osprey"]
