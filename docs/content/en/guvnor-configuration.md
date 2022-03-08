---
title: Guvnor Configuration
description: ''
position: 3
category: Guide
---

Guvnor has a global configuration file that can usually be found at `/etc/guvnor/config.yaml`. This file is used to control how Guvnor itself behaves, and how it configures Caddy.

Below is a configuration file using all available options:

```yaml
caddy:
  # image controls the Caddy container image that should be deployed,
  image: caddy:2.4.6-alpine
  acme:
    # ca controls which ACME provider should be used, this can be useful for switching to staging LetsEncrypt.
    ca: https://acme-v02.api.letsencrypt.org/directory
    # email specifies where the CA provider should contact you with information regarding the certificate
    email: support@example.com
  ports:
    http: 80
    https: 443

paths:
  # config is a path to where service configurations should be searched for
  config: /etc/guvnor/services
  # state is a path to where Guvnor will persist its state and history
  state: /var/lib/guvnor
```
