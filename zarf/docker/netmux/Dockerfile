FROM alpine:latest
ARG TARGETOS
ARG TARGETARCH
ADD ./zarf/docker/netmux/bin/${TARGETOS}/${TARGETARCH}/netmux /app/service

RUN apk add nmap tcpdump libpcap-dev curl openssh git

RUN chmod a+x /app/service

CMD "/app/service"