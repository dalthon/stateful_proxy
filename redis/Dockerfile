FROM redis:alpine

WORKDIR /home/redis

COPY config.sh      /home/redis/config.sh
COPY create-cluster /home/redis/create-cluster
COPY entrypoint.sh  /home/redis/entrypoint.sh

RUN apk update                       && \
    apk upgrade                      && \
    apk add --update --no-cache bash && \
    rm -rf /var/cache/apk/*

ENTRYPOINT ["/home/redis/entrypoint.sh"]

CMD ["./create-cluster", "tailall"]
