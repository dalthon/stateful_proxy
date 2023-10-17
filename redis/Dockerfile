FROM redis:7-alpine

WORKDIR /home/redis

RUN apk update                       && \
    apk upgrade                      && \
    apk add --update --no-cache bash && \
    rm -rf /var/cache/apk/*

COPY config.sh      /home/redis/config.sh
COPY create-cluster /home/redis/create-cluster
COPY entrypoint.sh  /home/redis/entrypoint.sh

ENTRYPOINT ["/home/redis/entrypoint.sh"]

CMD ["./create-cluster", "tailall"]
