ARG NAUTOBOT_VERSION=3.2.5
FROM ghcr.io/nautobot/nautobot:${NAUTOBOT_VERSION}
ARG NAUTOBOT_VERSION
RUN pip3 install --no-cache-dir \
      "nautobot[remote_storage]==${NAUTOBOT_VERSION}" \
      "nautobot-bgp-models==3.1.1" \
      boto3 \
 && nautobot-server --version
