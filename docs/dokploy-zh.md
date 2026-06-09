# Dokploy 部署说明

本文说明在 [Dokploy](https://dokploy.com) 上部署本仓库时的常见坑与推荐做法。若域名无法打开，请按顺序排查。

## 1. 内部端口必须是 3000

本应用默认监听 **3000**（可通过环境变量 `PORT` 修改；Dockerfile 已设置 `ENV PORT=3000`）。

在 Dokploy 中为该服务绑定域名时，**「内部端口 / Container Port」必须填 `3000`**。若填成 `80`、`8080` 等，Traefik 会把流量打到错误端口，表现为超时、502 或「访问不到」。

仓库根目录提供 **`docker-compose.dokploy.yml`**：`new-api` 使用 `expose: "3000"`，便于与 Traefik 对齐，且不把数据库暴露到公网。

## 2. 网络：加入 `dokploy-network`

使用 Docker Compose 时，对外提供 Web 的服务必须加入 Dokploy 创建的外部网络 **`dokploy-network`**，否则 Traefik 无法路由到容器。

`docker-compose.dokploy.yml` 已包含：

```yaml
networks:
  dokploy-network:
    external: true
```

若你自建 compose，请自行补上该网络并把 `new-api` 挂上去。

## 3. 应用监听地址

进程应监听 **`0.0.0.0`**（所有接口），而不是仅 `127.0.0.1`。当前 `main.go` 已使用 `0.0.0.0:<PORT>`，避免在部分编排环境下只绑定回环地址。

## 4. MySQL 与 `SQL_DSN`

- Compose 里 MySQL 服务名为 **`mysql`** 时，`SQL_DSN` 主机名也应为 `mysql`，例如：  
  `root:<密码>@tcp(mysql:3306)/new-api?charset=utf8mb4`
- 使用参考 compose 时，请在 Dokploy 中设置 **`MYSQL_DATABASE=new-api`**（compose 已写死）并保证 **`MYSQL_ROOT_PASSWORD`** 与 DSN 里密码一致。
- 若配置了 **`REDIS_CONN_STRING`**，必须保证 Redis 真的可达，否则进程会在启动时因 Ping 失败退出。

## 5. DNS

域名（例如 `llm.example.com`）的 **A/AAAA 记录** 应指向 Dokploy 所在服务器的公网 IP。仅在内网解析或未生效时，外网会「访问不到」。

## 6. 凭据安全

数据库密码、会话密钥等只应放在 Dokploy 环境变量或密钥管理中，**不要写入仓库**。若密码曾泄露，请在面板中修改并同步更新 `SQL_DSN`。

## 快速自检

1. Dokploy 容器日志：应用是否已打印启动成功、是否在连库/Redis 处退出。  
2. 域名配置中的 **服务名** 是否对应提供 Web 的 service（如 `new-api`），**端口** 是否为 **3000**。  
3. 本机 `curl -sS -o /dev/null -w "%{http_code}" http://<服务器IP>:<映射端口>/api/status`（若做了主机端口映射）是否为 200。
