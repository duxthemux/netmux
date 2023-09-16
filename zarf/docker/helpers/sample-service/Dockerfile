FROM alpine:latest

RUN apk add curl
COPY ./zarf/docker/helpers/sample-service/service /app/service
CMD "/app/service"