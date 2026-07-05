FROM kalilinux/kali-rolling:latest

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    tcpdump \
    nmap \
    iproute2 \
    ca-certificates \
    iputils-ping \
    && rm -rf /var/lib/apt/lists/*

CMD ["/bin/bash", "-lc"]
