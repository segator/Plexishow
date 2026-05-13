FROM alpine:3.19
RUN apk add --no-cache ffmpeg ca-certificates
COPY bin/plexishow /usr/local/bin/plexishow
EXPOSE 8080
ENTRYPOINT ["plexishow"]
CMD ["-config", "/etc/plexishow/config.yaml"]
