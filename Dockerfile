FROM alpine:3.21
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    libva \
    mesa-va-gallium \
    libdrm \
    && if [ "$TARGETARCH" = "amd64" ]; then \
        apk add --no-cache libva-intel-driver intel-media-driver; \
    fi
COPY bin/plexishow /usr/local/bin/plexishow
COPY assets/ /assets/
EXPOSE 8080
ENTRYPOINT ["plexishow"]
CMD ["-config", "/etc/plexishow/config.yaml"]
