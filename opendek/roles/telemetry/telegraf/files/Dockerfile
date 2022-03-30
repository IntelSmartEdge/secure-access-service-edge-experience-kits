# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation

ARG TELEGRAF_TAG
FROM telegraf:$TELEGRAF_TAG AS builder

RUN apk update && \
    apk add --update --no-cache \
    build-base=~0.5-r2 \
    bash=~5.1.8-r0 \
    sudo=~1.9.8_p2-r1 \
    git=~2.34.1-r0 \
    alpine-sdk=~1.0-r1 \
    libffi-dev=~3.4.2-r1 \
    openssl-dev=~1.1.1n-r0 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR "/telegraf_docker/"

COPY ./images/telegraf/script/install_intel_cmt_cat.sh ./

RUN ./install_intel_cmt_cat.sh

# Build final image where telegraf is running.
FROM alpine:3.15.0

RUN export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" && \
    apk update && \
    apk add --update --no-cache \
    util-linux=~2.37.4-r0 \
    bash=~5.1.16-r0 \
    sudo=~1.9.8_p2-r1 \
    smartmontools=~7.2-r1 \
    nvme-cli=~1.16-r0 \
    rasdaemon=~0.6.7-r0 \
    iptables=~1.8.7-r1 \
    ipmitool=~1.8.18-r10 \
    su-exec=~0.2-r1 \
    iputils=~20210722-r0 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR "/telegraf_docker/"

COPY ./images/telegraf/script/sudo_configuration.sh .

# Copy dependecies from previous step called builder, where intel-cmt-cat is installed.
COPY --from=builder /usr/local/lib/libpqos.* /usr/lib/
COPY --from=builder /usr/local/bin/ /usr/local/bin/
COPY --from=builder /usr/share/snmp/mibs/ /usr/share/snmp/mibs/
COPY --from=builder /usr/bin/telegraf /usr/bin/telegraf
COPY --from=builder entrypoint.sh .

# Set sudo config for specific plugins, and just in case update alternatives, because sometimes docker can mess it up.
RUN ./sudo_configuration.sh

# Override entrypoint
ENTRYPOINT ["sh", "-c", "rasdaemon -r && sleep 10s && ./entrypoint.sh telegraf"]
