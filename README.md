# NFT 交易系统

## 简介

基于区块链的CS饰品交易系统

## 运行

### 环境要求

- Go 1.23+
- Node.js 20+
- npm 9+
- Docker
- Docker Compose
- PostgreSQL

### 1. 启动网络

第一次启动：
```bash
cd network
./install.sh
```
重启：
```bash
cd network
./restart.sh
```
卸载：
```bash
cd network
./uninstall.sh
```
### 2. 启动后端


```bash

cd application/server
go run main.go
# 如需steam代理,添加环境变量STEAM_HTTP_PROXY
```

### 3. 启动前端服务


```bash
cd application/web
npm install
npm run dev
```
