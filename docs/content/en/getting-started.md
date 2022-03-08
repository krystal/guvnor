---
title: Getting started
description: ''
position: 2
category: Guide
---

## Installation

Installating Guvnor is relatively simple, you can either leverage our installation script or simply download the binary from our release page and place it somewhere in your path.

After adding the binary to your path, you'll need to run `guvnor init` in order to set up the default configuration for that version of Guvnor.

*For example:*

```bash
# Use this handy script to download the binary and place it in your bin or 
# manually download the binary from our release page
curl https://guvnor.k.io/install.sh | sudo bash
# Run the init command to setup any default config
guvnor init
# You are now ready to go !
```

## Deploying your first service
