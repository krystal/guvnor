---
title: Getting started
description: ''
position: 2
category: Guide
---

## Installation

These manual steps are temporary.

1. Download the binary from the releases page, or compile from source.
2. Create `/etc/guvnor` and a `/etc/guvnor/config.yaml` similar to:

    ```yaml
    caddy:
      image: docker.io/library/caddy:2.4.6-alpine
    paths:
      config: /etc/guvnor/services
      state: /var/lib/guvnor
    ```

3. Ensure the config & state paths you've specified exist.

## Deploying your first service
