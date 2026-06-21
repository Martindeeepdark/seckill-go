FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ARG SERVICE
COPY build/${SERVICE} /app/server
WORKDIR /app
ENTRYPOINT ["/app/server"]
CMD ["configs/config.docker.yaml"]
