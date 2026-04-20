FROM rust:bookworm AS builder
WORKDIR /home/rust/src

RUN apt-get update && apt-get install -y \
    cmake \
    clang \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth=1 https://github.com/redlib-org/redlib.git .
RUN git fetch origin pull/544/head:pr544
RUN git checkout pr544
RUN git merge pr544
RUN cargo build --release

FROM registry.opencode.de/open-code/oci/base:1.0.0-rc.5-minimal
COPY --from=builder --chown=53111:53111 /home/rust/src/target/release/redlib /home/nonroot
CMD ["/home/nonroot/redlib"]
