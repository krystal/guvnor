---
title: Service Configuration
description: ''
position: 3
category: Guide
fullscreen: true
---

Every service in Guvnor is represented by a YAML configuration file. This file has many options that let you configure which processes and tasks should be available as part of your service.

```yaml
# /etc/guvnor/services/identity.yaml

defaults:
  image: ghcr.io/krystal/identity
  imageTag: latest
  env:
    RAILS_ENV: production
    SECRET_KEY_BASE: abcdef1234567890
  mounts:
    - host: /opt/identity/config.yml
      container: /config.yml

processes:
  web:
    command: ["bin/rails", "server"]
    quantity: 1
    privileged: true
    env:
      HOSTNAME: identity.k.io
    caddy:
      hostnames:
        - identity.k.io
        - identity.another.domain
      path: /fizz/*

  worker:
    command: ["bin/rake", "worker"]
    quantity: 4

  cron:
    command: ["bin/rake", "cron"]
    network:
      mode: host

tasks:
  console:
    command: ["bin/rails", "console"]
    interactive: true

  migrate:
    command: ["bin/rake", "db:migrate"]

  notifySlack:
    image: slack
    imageTag: 21.3.4
    env:
      SLACK_CHANNEL: '#labs'
      SLACK_MESSAGE: "Do something {.Host}"

callbacks:
  preDeployment: [migrate]
  postDeployment: [notifySlack]
```

## Path routing

Path routing allows requests for certain paths to be directed to a different service. Path matching is case insensitive and exact by default, but wildcards can be used.

For example, to route the traffic for all paths and subpaths of `example.com/fizz`, configure the caddy option within your service as so:

```yaml
caddy:
  hostnames:
    - example.com
  path: /fizz/*
```

Matching precedence for paths is based on the number of segments in the path, so a service configured with `/fizz/buzz/*` will take precedence over `/fizz/*` which in turn has precedence over a service configured with just a hostname.