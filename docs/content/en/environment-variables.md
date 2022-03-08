---
title: Environment Variables
description: ''
position: 4
category: Guide
---

Containers that have been orchestrated using Guvnor will be provided several environment variables that provide context about the deployment to the running application.

These environment variables will depend on whether the container has been created as part of a Task or Process.

## Process

Process containers orchestrated by Guvnor will include the following environment variables:

- `GUVNOR_SERVICE`: The name of the service that the process is associated with
- `GUVNOR_PROCESS`: The name of the process that the container is associated with
- `PORT`: The port that the application should accept traffic on from Guvnor
- `GUVNOR_DEPLOYMENT`: The ID of the the deployment (an incrementing counter)

## Task

- `GUVNOR_SERVICE`: The name of the service that the process is associated with
- `GUVNOR_TASK`: The name of the task

### Task triggered by callback

When a task is triggered as part of a callback during deployment, further environment variables are also provided:

- `GUVNOR_DEPLOYMENT`: The ID of the the deployment (an incrementing counter)
- `GUVNOR_CALLBACK`: Either `PRE_DEPLOYMENT` or `POST_DEPLOYMENT`
