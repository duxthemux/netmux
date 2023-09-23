FROM alpine:latest
ARG TARGETOS
ARG TARGETARCH
RUN apk add nmap tcpdump libpcap-dev curl openssh git

ADD ./zarf/docker/netmux/bin/${TARGETOS}/${TARGETARCH}/netmux /app/service

CMD "/app/service"