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

## 7. 打开域名后出现站内「404 / Page Not Found」

常见原因是浏览器仍缓存**旧版** `index.html`，其中的 `/assets/*.js` 哈希与**新镜像**不一致，导致 JS 请求失败，前端无法正常启动。

处理建议：

1. 使用**无痕窗口**或做一次**强制刷新**（绕过缓存）再访问。  
2. 在浏览器开发者工具 **Network** 里查看 `/assets/` 下脚本是否为 **200**；若为 **404**，多半是缓存与版本不一致。  
3. 重新部署后可在 Dokploy 中勾选「清除构建缓存」或等待 CDN/浏览器缓存自然过期。

后端已调整：对缺失的 `/assets/*` 返回普通 **404**，不再返回 API 用的 JSON，避免浏览器把错误响应当成脚本解析。

## 8. 数据库里的「服务器地址」(ServerAddress)

- 面板/数据库中的键名必须是 **`ServerAddress`**（区分大小写），值例如：`https://llm.example.com`（不要末尾 `/`）。  
- 该字段用于 **OAuth 回调、邮件里的链接、Webhook 说明、Passkey 等**，**不会让 DNS 或 Traefik  magically 指向你的主机**；域名打不开时仍要查网络与 Dokploy 端口。  
- 修改 `options` 表后，进程会通过定时同步读到新值；若曾启用 Passkey 且遇到域名更换后仍异常，请部署 **含本修复的版本**（Passkey 推导不再锁死旧域名），或同时在后台把 **Passkey 的 RPID / Origins** 改成与新域名一致。

## 快速自检

1. Dokploy 容器日志：应用是否已打印启动成功、是否在连库/Redis 处退出。  
2. 域名配置中的 **服务名** 是否对应提供 Web 的 service（如 `new-api`），**端口** 是否为 **3000**。  
3. 本机 `curl -sS -o /dev/null -w "%{http_code}" http://<服务器IP>:<映射端口>/api/status`（若做了主机端口映射）是否为 200。
