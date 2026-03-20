#!/bin/bash
# Bytebase Backend Startup Script
#
# 说明:
#   - 使用外部 PostgreSQL (通过 PG_URL 指定)
#   - --data 参数仍需要，用于存储内存分析文件等运行时数据
#
# 编译方式:
#   cd /home/huanghe314/workspace/go/src/github.com/huanghe314/bytebase
#   go build -o bytebase-build/bytebase ./backend/bin/server/main.go

export PG_URL="postgresql://bytebase:bytebase@localhost:5432/bytebase?sslmode=disable"

./bytebase-build/bytebase \
  --data /home/huanghe314/workspace/go/src/github.com/huanghe314/bytebase/bytebase-data \
  --port 8080
