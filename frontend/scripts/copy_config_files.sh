#!/bin/sh
cp ../backend/common/permission/permission.yaml ./src/types/iam/
node ./scripts/generate_permissions.js
