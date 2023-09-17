FROM alpine:latest
ARG TARGETOS
ARG TARGETARCH

#RUN apk add curl

ADD ./zarf/docker/helpers/sample-service/bin/${TARGETOS}/${TARGETARCH}/service /app/service

CMD "/app/service"