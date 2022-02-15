<h1 align="center">
  guvnor
</h1>

<p align="center">
  <strong>
    Handy tool for deploying containerised apps on Linux hosts.
  </strong>
</h4>

<p align="center">
  <a href="https://github.com/krystal/guvnor/actions">
    <img src="https://img.shields.io/github/workflow/status/krystal/guvnor/CI.svg?logo=github" alt="Actions Status">
  </a>
  <a href="https://codeclimate.com/github/krystal/guvnor">
    <img src="https://img.shields.io/codeclimate/coverage/krystal/guvnor.svg?logo=code%20climate" alt="Coverage">
  </a>
  <a href="https://github.com/krystal/guvnor/commits/main">
    <img src="https://img.shields.io/github/last-commit/krystal/guvnor.svg?style=flat&logo=github&logoColor=white"
alt="GitHub last commit">
  </a>
  <a href="https://github.com/krystal/guvnor/issues">
    <img src="https://img.shields.io/github/issues-raw/krystal/guvnor.svg?style=flat&logo=github&logoColor=white"
alt="GitHub issues">
  </a>
  <a href="https://github.com/krystal/guvnor/pulls">
    <img src="https://img.shields.io/github/issues-pr-raw/krystal/guvnor.svg?style=flat&logo=github&logoColor=white" alt="GitHub pull requests">
  </a>
  <a href="https://github.com/krystal/guvnor/blob/main/MIT-LICENSE">
    <img src="https://img.shields.io/github/license/krystal/guvnor.svg?style=flat" alt="License Status">
  </a>
</p>

---

**WARNING:** This product is currently unstable ! We do not recommend its use
in production deployments.

---

## Installation

These manual steps are temporary.

1. Download the binary from the releases page, or compile from source.
2. Create `/etc/guvna` and a `/etc/guvna/config.yaml` similar to:
```yaml
caddy:
  image: docker.io/library/caddy:2.4.6-alpine
paths:
  config: /etc/guvnor/services
  state: /var/lib/guvnor
```
3. Ensure the config & state paths you've specified exist.