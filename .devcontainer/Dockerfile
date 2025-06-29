FROM ubuntu:22.04

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ENV LANG=C.UTF-8

RUN apt-get update --no-install-recommends \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
    && apt-get clean && rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/*

RUN apt-get update --no-install-recommends \
    && apt-get install -y --no-install-recommends \
        tzdata \
    && apt-get clean && rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/* \
    && ln -sf /usr/share/zoneinfo/Asia/Tokyo /etc/localtime \
    && echo 'Asia/Tokyo' >/etc/timezone

RUN apt-get update --no-install-recommends \
    && apt-get install -y \
        build-essential \
        curl \
        zip \
        git \
        ssh \
        bash-completion \
        sqlite3 \
    && apt-get clean && rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/*

ARG GO_VERSION=1.24.3
RUN curl "https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz" -L --output go.tar.gz \
    && tar xvf go.tar.gz -C /usr/local/ \
    && echo 'export PATH=$PATH:/usr/local/go/bin:/home/vscode/.local/bin' >> /etc/profile \
    && rm go.tar.gz

ARG NODE_VERSION=22.14.0
ARG NODE_HOME=/opt/node
ENV PATH="${NODE_HOME}/bin:${PATH}"
RUN --mount=type=cache,target=/var/cache/apt set -x \
    && apt-get update \
    && apt-get install -y \
    curl \
    xz-utils \
    && mkdir -p ${NODE_HOME} \
    && curl -fsSL https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz  -o /tmp/node.tar.xz \
    && tar -xJf /tmp/node.tar.xz -C /tmp \
    && cp -R /tmp/node-v${NODE_VERSION}-linux-x64/* ${NODE_HOME}/ \
    && rm -rf /tmp/node.tar.xz /tmp/node-v${NODE_VERSION}-linux-x64

RUN npm install -g @google/gemini-cli

RUN groupadd --gid 1000 vscode \
    && useradd --uid 1000 --gid 1000 -m vscode

USER vscode
ENV HOME /home/vscode
