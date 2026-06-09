# Dokploy 部署说明

本文说明在 [Dokploy](https://dokploy.com) 上部署本仓库时的常见坑与推荐做法。若域名无法打开，请按顺序排查。

## Dokploy 控制台必查（与业务代码无关）

以下与 [Dokploy 文档：Troubleshooting](https://docs.dokploy.com/docs/core/troubleshooting)、[Docker Compose Domains](https://docs.dokploy.com/docs/core/docker-compose/domains) 一致：

1. **必须在 Dokploy UI → 你的 Compose 应用 → Domains 里添加域名**，并选择服务名 **`new-api`**、内部端口 **`3000`**。若只保存了 compose、却**从未在 UI 里添加域名**，Dokploy **不会**注入 Traefik 路由，公网用域名访问会**一直失败**（文档明确：未配域名则不会对 compose 自动加 labels）。  
2. 部署前点 **Preview Compose**，确认最终执行的文件里，`new-api` 带有 `traefik.enable=true` 以及 `loadbalancer.server.port=3000`（或与你 `PORT` 一致）等配置。  
3. **先**在 DNS 把域名 **A 记录** 指到服务器公网 IP，**再**在 Dokploy 里添加域名并签证书；顺序反了 Let's Encrypt 可能失败，需删域名重加或按文档处理 Traefik。  
4. 一般**不要**在 Dokploy「高级」里给 Web 服务再绑主机 **Ports**（除非你要 `IP:端口` 直连），以免与 Traefik 的 80/443 冲突。  
5. Compose 里若 **healthcheck 一直失败**，官方说明可能导致**域名永远不通**；本仓库示例里 `new-api` 依赖 **MySQL healthy**，若 MySQL 密码/健康检查不对，`new-api` 可能**一直起不来**，Traefik 也无后端可连。  
6. 容器挂在**多个 Docker 网络**时，Traefik 可能连到错误的网卡；`docker-compose.dokploy.yml` 已为 `new-api` 设置 **`traefik.docker.network=dokploy-network`**。  
7. 使用 Compose / 模板时，**每次在 UI 里改域名后都要重新 Deploy** 才会生效。

## 1. Dokploy 里填 3060 还是 3000？（最常见误配）

根目录 **`docker-compose.yml`** 里有：

```yaml
ports:
  - "${HOST_PORT:-3060}:3000"
```

含义是：

| 端口 | 含义 | 谁在用 |
|------|------|--------|
| **3060** | **宿主机**上映射出来的端口 | 你在自己电脑上访问 `http://服务器IP:3060` 时走这条 |
| **3000** | **容器内** new-api 进程真正监听的端口 | `Dockerfile` 里 `ENV PORT=3000`，进程 `Listen 0.0.0.0:3000` |

**Dokploy 的 Traefik 和容器在同一个 Docker 网络里**，它会把流量直接转到 **`new-api` 容器的某个端口**，**不会**先绕到宿主机的 `3060`。  
因此在 **Domains** 里填的「内部端口 / Container Port」必须是 **3000**（除非你显式把环境变量 `PORT` 改成了别的，并与之一致）。

若填 **3060**：Traefik 会去连 `new-api:3060`，而容器里**没有**进程监听 3060 → 典型表现是 **502 / Bad Gateway / 连接被拒绝**。你改成 3000「仍不好使」时，请再核对：**服务名是否为 `new-api`、是否已 Preview Compose 看到 `loadbalancer.server.port=3000`、MySQL 是否 healthy 导致容器根本没起来**。

仓库里的 **`docker-compose.dokploy.yml`** 使用 `expose: "3000"`、不设 `ports: 3060:3000`，就是为了和 Dokploy/Traefik 的语义一致，避免和本机映射端口混淆。

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
