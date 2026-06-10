# Dokploy 部署说明

本文说明在 [Dokploy](https://dokploy.com) 上部署本仓库时的常见坑与推荐做法。若域名无法打开，请按顺序排查。

## Dokploy 控制台必查（与业务代码无关）

以下与 [Dokploy 文档：Troubleshooting](https://docs.dokploy.com/docs/core/troubleshooting)、[Docker Compose Domains](https://docs.dokploy.com/docs/core/docker-compose/domains) 一致：

1. **必须在 Dokploy UI → 你的 Compose 应用 → Domains 里添加域名**，并选择服务名 **`new-api`**、内部端口 **`3000`**。若只保存了 compose、却**从未在 UI 里添加域名**，Dokploy **不会**注入 Traefik 路由，公网用域名访问会**一直失败**（文档明确：未配域名则不会对 compose 自动加 labels）。  
2. 部署前点 **Preview Compose**，确认最终执行的文件里，`new-api` 带有 `traefik.enable=true` 以及 `loadbalancer.server.port=3000`（或与你 `PORT` 一致）等配置。  
3. **先**在 DNS 把域名 **A 记录** 指到服务器公网 IP，**再**在 Dokploy 里添加域名并签证书；顺序反了 Let's Encrypt 可能失败，需删域名重加或按文档处理 Traefik。  
4. 一般**不要**在 Dokploy「高级」里给 Web 服务再绑主机 **Ports**（除非你要 `IP:端口` 直连），以免与 Traefik 的 80/443 冲突。  
5. Compose 里若 **healthcheck 一直失败**，官方说明可能导致**域名永远不通**；本仓库示例里 `new-api` 依赖 **MySQL healthy**，且 **`new-api` 自带 `/api/status` 健康检查**（与镜像 Dockerfile 一致）。若 MySQL 密码/健康检查不对，或应用连不上库，`new-api` 可能一直 **unhealthy**，面板里会看到红色状态。  
6. **不要在 compose 里手写 `traefik.docker.network=dokploy-network` 并强挂 `dokploy-network`**（除非你明确走「手动 Traefik 标签」且关闭隔离部署）：与 Dokploy「隔离部署」生成的网络名容易冲突，导致 502 / 路由异常。仓库 **`docker-compose.dokploy.yml`** 已改为只保留 `new-api-internal`（连 MySQL），**公网路由网络由 Dokploy 在绑定 Domains 后自动注入**；请以 **Preview Compose** 为准。  
7. 使用 Compose / 模板时，**每次在 UI 里改域名后都要重新 Deploy** 才会生效。
8. **先分清「Traefik 404」和「应用内 404」**（见下文 **§0**）。若页面上只有**一行纯英文** `404 page not found`、没有站点标题/样式/大号数字，这几乎一定是 **Traefik 没有匹配到路由**，请求**从未进入** `new-api` 容器；此时去改数据库里的 `ServerAddress` 或应用代码都**解决不了**，必须在 Dokploy / Traefik 侧把域名路由指到本服务。

## 0. 纯文字「404 page not found」= Traefik，不是本仓库前端

Traefik 在「当前入口点下没有任何 HTTP 路由匹配该请求的 Host/Path」时，会返回 **HTTP 404**，响应体为固定纯文本 **`404 page not found`**（常见 `Content-Type: text/plain`）。这与本仓库 **TanStack** 里带排版、按钮和「Oops! Page Not Found!」文案的页面**不是同一种现象**。

**建议自检：**

```bash
# 看响应头与类型：若为 text/plain 且体积极小，多为 Traefik 未路由到后端
curl -sS -D - -o /dev/null "https://你的域名/" | head -25
```

**处理方向：** 回到本文开头「控制台必查」第 1–2 步——在 **Domains** 里添加**与浏览器地址栏完全一致**的域名（含/不含 `www` 要一致）、服务名 **`new-api`**、内部端口 **`3000`**，**Preview Compose** 确认已出现 `traefik.enable=true` 及 `loadbalancer.server.port=3000`，改完后**重新 Deploy**。若用 IP 访问或未带正确 `Host` 头，Traefik 也常返回上述默认 404。

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

## 2. 网络：Traefik 与 MySQL 分离

- **`docker-compose.dokploy.yml`**：`new-api` 与 `mysql` 仅共享 **`new-api-internal`**（MySQL 不暴露公网）。**不在此文件声明 `dokploy-network`**：你在 Dokploy **Domains** 里绑定域名并选择 **`new-api`** 后，Dokploy 会把 Traefik 所需的网络与 `traefik.*` 标签**自动合并进最终 compose**（见官方 [Domains](https://docs.dokploy.com/docs/core/docker-compose/domains)）。用 **Preview Compose** 确认 `new-api` 最终带有 Traefik 相关 `networks` / `labels` 即可。  
- **自建 compose（不用本参考文件）**时：若未开启隔离部署，通常仍需把对外 Web 服务挂到外部网络 **`dokploy-network`**，并自行配置 Traefik 标签或仍用 Domains 自动生成；若开启隔离部署，以 Dokploy 文档为准，避免手写死 `dokploy-network` 与 `traefik.docker.network` 与真实注入网络不一致。

## 3. 应用监听地址

进程应监听 **`0.0.0.0`**（所有接口），而不是仅 `127.0.0.1`。当前 `main.go` 已使用 `0.0.0.0:<PORT>`，避免在部分编排环境下只绑定回环地址。

## 4. MySQL 与 `SQL_DSN`

- Compose 里 MySQL 服务名为 **`mysql`** 时，`SQL_DSN` 主机名也应为 `mysql`，例如：  
  `root:<密码>@tcp(mysql:3306)/new-api?charset=utf8mb4`
- 使用参考 compose 时，请在 Dokploy 中设置 **`MYSQL_DATABASE=new-api`**（compose 已写死）并保证 **`MYSQL_ROOT_PASSWORD`** 与 DSN 里密码一致。
- 若配置了 **`REDIS_CONN_STRING`**，必须保证 Redis 真的可达，否则进程会在启动时因 Ping 失败退出。
- **环境变量模板**：[`docs/env.dokploy.example`](./env.dokploy.example) 汇总了 Dokploy 面板里常用必填/可选项，可与根目录 [`.env.example`](../.env.example) 对照。

## 4a. `docker-compose.dokploy.yml` 内建项（便于运维）

| 项 | 说明 |
|----|------|
| `new-api` / `mysql` 的 `logging` | `json-file` 单文件最大 50MB、保留 3 个文件，避免占满磁盘。 |
| `new-api` `healthcheck` | 轮询 `http://127.0.0.1:3000/api/status`，与镜像 `HEALTHCHECK` 行为一致，便于在 Dokploy 里看容器健康度。 |
| `stop_grace_period` | 给进程一点优雅退出时间（尤其有长连接时）。 |
| 注释掉的 `ports` | 需要 **SSH 隧道 + 本机 3060** 排错时可取消注释 `127.0.0.1:3060:3000`；公网访问仍建议只走 Traefik。 |

## 5. DNS

域名（例如 `llm.example.com`）的 **A/AAAA 记录** 应指向 Dokploy 所在服务器的公网 IP。仅在内网解析或未生效时，外网会「访问不到」。

## 6. 凭据安全

数据库密码、会话密钥等只应放在 Dokploy 环境变量或密钥管理中，**不要写入仓库**。若密码曾泄露，请在面板中修改并同步更新 `SQL_DSN`。

## 6a. 「IP + 宿主端口」能访问，但域名仍不通（最常见：没走同一条链路）

`http://服务器公网IP:3060`（或你在 `ports` 里映射的端口）能打开，只说明 **容器进程在监听、且宿主机端口已映射**；**通过域名访问**时，浏览器默认走 **80 / 443**，由 **Dokploy 自带的 Traefik** 再转发到容器，**不会**经过你映射出来的那个宿主端口。

请按顺序核对：

1. **本机与云厂商安全组是否放行 80、443**  
   只放行 `3060` 而没放行 `80/443` 时，会出现「IP+端口能开、https 域名连不上 / 超时」。
2. **DNS**  
   域名 **A（或 AAAA）记录** 是否指向**当前这台 Dokploy 服务器**的公网 IP；若走 CDN/代理，需确认回源与证书是否与 Dokploy 一致。
3. **Preview Compose（必做）**  
   在 Dokploy 里打开 **Preview Compose**，在最终内容里确认：  
   - 你为域名选中的 **service 名称**（`services:` 下的 key，例如 `new-api`）与 **Domains** 里选的一致；  
   - 已出现 `traefik.enable=true` 以及 `traefik.http.services....loadbalancer.server.port=3000`（或与你 `PORT` 一致）；  
   - `new-api` 所在网络里**至少有一个** Traefik 能加入的网络（非隔离部署时常见为 `dokploy-network`）。  
   若 Preview 里**完全没有**这些 Traefik 相关行，说明 **Domains 未真正生效** 或 **未选对该 Compose 服务**，需要重新保存域名并 **Redeploy**。
4. **隔离部署 (Isolated Deployments)**  
   若你曾按旧教程在 compose 里手写 **`traefik.docker.network=dokploy-network`** 或强挂 **`dokploy-network`**，可能与隔离模式下的**项目专有网络**不一致，表现为 **502 / 无路由**。请改用仓库当前 **`docker-compose.dokploy.yml`**（已去掉上述手写项），或自行删除冲突的 `labels` / `networks` 后 **Redeploy**。
5. **在服务器上带 Host 头自测（区分 DNS 与 Traefik）**  
   在 Dokploy 机器上执行（将域名换成你的）：  
   `curl -sS -o /dev/null -w "%{http_code}\n" -H "Host: llm.example.com" http://127.0.0.1/`  
   - 若得到 **404** 且响应体为纯文字 `404 page not found`：Traefik 仍未匹配到该 Host 的路由 → 回到第 3、4 步。  
   - 若得到 **301/302/200**：本机 Traefik 已匹配，问题更可能在 **公网 DNS / 防火墙 / 证书**。

## 7. 打开域名后出现**应用内**「404 / Oops! Page Not Found」（带样式的页面）

本节针对 **已能加载到本站点 HTML/JS**（页面有正常布局、大号「404」与「Oops! Page Not Found!」等）的情况。若你看到的是**纯一行** `404 page not found`，请见上文 **§0**（Traefik 未路由）。

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

## 9. 操练场报错 `openai_error` / `bad_response_status_code`（模型名与上游不一致）

操练场走 **OpenAI 兼容** 接口，请求里的 **`model` 字段会原样（或经渠道「模型重定向」后）发给上游**。若后台渠道里填的模型 ID 与上游服务实际支持的名称不一致，上游会返回 **4xx/5xx**，本网关会包装为 `type: openai_error`、`code: bad_response_status_code`。

**处理要点：**

1. 在 **渠道 → 模型列表** 中使用上游文档要求的 ID（例如 dopio 侧为 **`sd2-c1` / `sd2-c2` / `sd2-c3`**），与操练场下拉框所选名称一致。  
2. 若对外展示名与上游 ID 不同，可在渠道配置 **模型重定向**（`model_mapping`），例如：`{"展示用别名":"sd2-c2"}`。  
3. 内置默认单价与按次计费名单已对齐 **`sd2-c1/c2/c3`**。若你在环境变量 **`TASK_PRICE_PATCH`** 里曾写过旧名 `seedance2-c*`，请改为新名或删除该项以使用内置默认。  
4. 仍失败时在 **渠道日志 / 上游响应** 中查看具体 HTTP 状态码与 body，核对 API Key、额度与模型是否在供应商侧已开通。

## 快速自检

1. Dokploy 容器日志：应用是否已打印启动成功、是否在连库/Redis 处退出。  
2. 域名配置中的 **服务名** 是否对应提供 Web 的 service（如 `new-api`），**端口** 是否为 **3000**。  
3. 本机 `curl -sS -o /dev/null -w "%{http_code}" http://<服务器IP>:<映射端口>/api/status`（若做了主机端口映射）是否为 200。  
4. **公网 80/443** 是否可达（与第 3 步的「宿主映射端口」不是同一条链路）；见上文 **§6a**。  
5. **Preview Compose** 是否已包含 Dokploy 注入的 Traefik labels。
