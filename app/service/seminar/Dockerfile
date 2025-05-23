FROM golang:alpine AS builder

COPY . /src
WORKDIR /src
RUN GOPROXY=https://goproxy.cn make build

FROM debian:stable-slim

RUN echo "deb http://mirrors.tuna.tsinghua.edu.cn/debian/ stable main contrib non-free non-free-firmware\ndeb http://mirrors.tuna.tsinghua.edu.cn/debian-security stable-security main contrib non-free non-free-firmware" > /etc/apt/sources.list

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates  \
        netbase \
        tini \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

RUN mkdir -p /app
COPY --from=builder /src/bin/server /app
COPY --from=builder /src/configs /app/configs

WORKDIR /app

ENTRYPOINT ["/usr/bin/tini", "--"]

EXPOSE 9002
CMD ["./server"]
