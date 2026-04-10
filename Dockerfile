FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY trvl /usr/local/bin/trvl
ENTRYPOINT ["trvl"]
