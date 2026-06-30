FROM ghcr.io/c4illin/convertx:v0.18.0
USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    intel-media-va-driver \
    libigdgmm12 && \
    rm -rf /var/lib/apt/lists/*
USER 33
