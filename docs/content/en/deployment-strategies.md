---
title: Deployment Strategies
description: ''
position: 4
category: Guide
---

## Default

This strategy is ideal for web serving processes, as it directs traffic towards the new replica before killing the old one. This removes any downtime from the rollout process.

1. Start new replica of process
2. Wait for that replica to become healthy
3. Direct traffic towards the new replica
4. Send SIGTERM to an old replica of process
5. Repeat until the count of new replicas meets the specified quantity

## Replace

This strategy is ideal for cronjobs, and other processes where you only want a single replica running at any one time.

1. Remove an existing replica of the process from the loadbalancer
2. Send SIGTERM to the old replica
3. Wait for it stop, kiling it after X seconds if not stopped.
4. Start new replica of the process
5. Wait for it to become healthy
6. Direct traffic towards the new replica
7. Repeat until the count of new replicas meets the specified quantity
