# new-api 生图接口文档（文生图 / 图生图）

> 实测环境：`domiex-llm-x4xe86-new-api-1`（容器内 `:3000`，未对宿主机发布端口）
> 上游渠道：**channel 52 「Image Omni」**（type 1 = OpenAI 兼容）→ `http://domiex-air-yw9hej-air2api-1:38474`
> 计费：**每次 25000 quota**（`air2apiImagePrice = 0.05`），按次计费，不按 token
> 任务超时：`TASK_TIMEOUT_MINUTES=600`
>
> 本文所有请求/响应均为**实跑结果**，非推测。文档中出现的任务 `task_xxx` 是真实任务 ID。

---

## 目录

1. [核心结论](#1-核心结论)
2. [鉴权与寻址](#2-鉴权与寻址)
3. [可用模型](#3-可用模型)
4. [接口总览](#4-接口总览)
5. [文生图（text-to-image）](#5-文生图text-to-image)
6. [图生图（image-to-image）](#6-图生图image-to-image)
7. [轮询与结果获取](#7-轮询与结果获取)
8. [参数速查表](#8-参数速查表)
9. [错误码与排错](#9-错误码与排错)
10. [完整可运行示例](#10-完整可运行示例)
11. [历史数据与性能基线](#11-历史数据与性能基线)
12. [已知问题](#12-已知问题)

---

## 1. 核心结论

**所有生图请求都是异步的，且强制异步。** 这一点很重要，和 OpenAI 原生行为不同：

- `POST /v1/images/generations` 或 `/v1/images/edits` **立即返回 HTTP 202**，body 是一个 `task` 对象，**不含图片**。
- 即使你不传 `async: true`，也一样返回 202 + task。实测：

```bash
# 不传任何 async 参数，仍然返回任务而非图片
curl -s -X POST .../v1/images/generations \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"nano-banana-2","prompt":"a blue cat","n":1}'
# → http=202  time=0.046s
# {"created":...,"id":"task_gLyfwhZoSF7XnSk1nEV0owzetq0rXFfE","kind":"image_generation",
#  "object":"task","status":"queued","taskId":"...","task_url":"/v1/images/generations/task_..."}
```

- **`response_format: "b64_json"` 会被忽略**，依然返回 task_id。想拿图必须走轮询。
- 图片实际耗时 **45–707 秒**（均值 124–140 秒）。所以同步等待不可行，**必须实现轮询**。

> 触发异步的显式参数（三者任一即可，但实际不传也是异步）：`async: true`、`async_task: true`、`return_task_id: true`。
> 代码位置：`controller/image_async.go` → `imageAsyncRequested()`。

---

## 2. 鉴权与寻址

**Base URL**

| 场景 | URL |
|---|---|
| 容器内（其它 docker 服务调用） | `http://domiex-llm-x4xe86-new-api-1:3000` |
| 同一 dokploy network 内 | `http://new-api:3000` 或按容器名 |
| 宿主机 | ⚠️ **未发布端口**。宿主 `:3000` 是 Dokploy，不是 new-api |

**鉴权**：`Authorization: Bearer <token>`，token 是 new-api 的 token（48 字符），不是上游 key。

```http
Authorization: Bearer QK8PS2TB0U3TTUZQguI7cMbrOGflch7JFoS3eYtOLG8aHhif
Content-Type: application/json
```

**分组（group）**：模型能否路由取决于 token 所属 group。channel 52 挂在
`default,vip,svip,vip1,vip2,vip3,vip6`。token 的 group 必须在其中，否则报
`model_not_found`。

查询 token 与 group：

```bash
docker exec domiex-clickhouse-5evaye-clickhouse-1 clickhouse-client \
  --user default --password <pw> -d new_api \
  -q "select id, name, key, \`group\` from tokens where status=1"
```

---

## 3. 可用模型

channel 52（Image Omni）提供 6 个模型。全部**同价 25000 quota/次**。

| 模型 ID | 实测用途 | 备注 |
|---|---|---|
| `gpt-image-2` | 文生图 / 图生图 | 最常用，2048² 级输出 |
| `gpt-image-2.5-sunburst` | 图生图 | **图生图主力**（历史 edit 记录最多） |
| `gpt-image-2.5-flare` | 文生图 | — |
| `nano-banana-2` | 文生图 / 图生图 | 历史文生图第二多 |
| `nano-banana-2-lite` | 文生图 | — |
| `nano-banana-pro` | 文生图 / 图生图 | **历史最常用**（86 次文生图 + 35 次图生图） |

列出模型：

```bash
curl -s http://domiex-llm-x4xe86-new-api-1:3000/v1/models \
  -H "Authorization: Bearer $KEY"
# → 该 token 可见 17 个模型，其中生图相关 6 个
```

---

## 4. 接口总览

| 方法 | 路径 | 作用 | 实测状态码 |
|---|---|---|---|
| POST | `/v1/images/generations` | 提交任务（文生图 / 图生图JSON） | **202** |
| POST | `/v1/images/edits` | 提交任务（图生图 multipart） | **202** |
| GET | `/v1/images/generations/{task_id}` | 查询文生图任务 | 200 |
| GET | `/v1/images/edits/{task_id}` | 查询图生图任务 | 200 |

**关于 `task_url` 的坑：**

- 提交文生图 → `task_url` = `/v1/images/generations/{task_id}`
- 提交 `/v1/images/edits` → `task_url` = `/v1/images/edits/{task_id}`
- **但 `POST /v1/images/generations` 带 `image_url`（图生图走 JSON）时，`kind` 仍是 `image_generation`，`task_url` 仍指向 `generations`** —— 此时用哪个路径查都能查到（`ImageTaskFetch` 按 task_id 查库，不校验路径前缀）。

> 实践建议：**永远用提交时返回的 `task_url` 去轮询**，不要自己拼。

---

## 5. 文生图（text-to-image）

### 5.1 提交

```bash
curl -s -X POST "http://domiex-llm-x4xe86-new-api-1:3000/v1/images/generations" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "a red apple on a wooden table, studio light",
    "n": 1,
    "size": "1024x1024",
    "async": true
  }'
```

### 5.2 真实响应（HTTP 202）

```json
{
  "created": 1789898811,
  "id": "task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "kind": "image_generation",
  "object": "task",
  "status": "queued",
  "taskId": "task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "task_id": "task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "task_url": "/v1/images/generations/task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "updated": 1789898811
}
```

> `id` / `task_id` / `taskId` 三个字段值相同，冗余提供以兼容不同客户端。

### 5.3 轮询到完成（实测 11 次轮询，约 146 秒）

```
poll 1:  running 30%
poll 2:  running 30%
...
poll 10: running 30%
poll 11: succeeded 100%
```

```json
{
  "created": 1789898811,
  "id": "task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "kind": "image_generation",
  "object": "task",
  "progress": "100%",
  "result": {
    "created_at": 1789898811,
    "finished_at": 1789898957,
    "id": "8e3b19ca-dd26-4e41-a3c1-46ec5d0573ba",
    "image_url": "/outputs/task_8e3b19ca-dd26-4e41-a3c1-46ec5d0573ba.png",
    "model": "gpt-image-2",
    "object": "task",
    "progress": 100,
    "prompt": "a red apple on a wooden table, studio light",
    "status": "completed",
    "task_id": "task_8e3b19ca-dd26-4e41-a3c1-46ec5d0573ba",
    "type": "image",
    "updated_at": 1789898957
  },
  "status": "succeeded",
  "taskId": "task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "task_id": "task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "task_url": "/v1/images/generations/task_BEf7o1CyZyi9cgXuXhC429bQjYpDCqvT",
  "updated": 1789898959
}
```

**图片 URL 有两种取法**（同一张图）：

| 取法 | URL |
|---|---|
| 相对路径（推荐） | `/outputs/task_8e3b19ca-...png` |
| 换 new-api host | `http://domiex-llm-x4xe86-new-api-1:3000/outputs/task_8e3b19ca-...png` |
| 换上游 host | `http://domiex-air-yw9hej-air2api-1:38474/outputs/task_8e3b19ca-...png` |

两者实测均返回 **HTTP 200**。文件是 PNG，本例 **3840×3840**，约 15 MB。

> 注意：`image_url` 是**相对路径**，指向 new-api/上游的 `/outputs/` 静态目录，
> 不是公网地址。若要给外部使用，需自行拼接可访问的 base URL 或加反代。

---

## 6. 图生图（image-to-image）

图生图有 **两条互不等价的通路**，按参考图传法自动分流：

| 通路 | 触发条件 | 上游路径 | `kind` |
|---|---|---|---|
| A. JSON | `POST /v1/images/generations` 且带参考图字段 | `/v1/images/generations` | `image_generation` |
| B. Multipart | `POST /v1/images/edits` 且 `Content-Type: multipart/form-data` | `/v1/images/edits` | `image_edit` |

分流逻辑在 `controller/image_async.go:runImageAsyncTask()`：`task.Action == "edit_image"` 走上游 `/v1/images/edits`，否则走 `/v1/images/generations`。

### 6.1 通路 A：JSON + image_url（参考图是 URL）

```bash
curl -s -X POST "http://domiex-llm-x4xe86-new-api-1:3000/v1/images/generations" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "turn the apple green, keep everything else identical",
    "n": 1,
    "image_url": "http://domiex-air-yw9hej-air2api-1:38474/outputs/task_8e3b19ca-....png",
    "async": true
  }'
```

**真实响应：**

```json
{
  "created": 1789898983,
  "id": "task_Iws2YbROBxwwkZzHWK7cgfLjSzAMp4cb",
  "kind": "image_generation",
  "object": "task",
  "status": "queued",
  "taskId": "task_Iws2YbROBxwwkZzHWK7cgfLjSzAMp4cb",
  "task_url": "/v1/images/generations/task_Iws2YbROBxwwkZzHWK7cgfLjSzAMp4cb"
}
```

**最终结果**（9 次轮询，109 秒）：

```json
{
  "progress": "100%",
  "result": {
    "finished_at": 1789899092,
    "image_url": "/outputs/task_dc097b36-5139-4055-a485-d7122c582e43.png",
    "model": "gpt-image-2",
    "prompt": "turn the apple green, keep everything else identical",
    "status": "completed",
    "task_id": "task_dc097b36-5139-4055-a485-d7122c582e43"
  },
  "status": "succeeded",
  "task_id": "task_Iws2YbROBxwwkZzHWK7cgfLjSzAMp4cb"
}
```

**✅ 视觉验证**：输出图确认苹果变绿、构图/光影/木纹与参考图一致 —— 图生图生效。
输出 **3840×2146**。

**参考图字段的多种写法**（`dto/openai_image.go` 全部接受）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `image_url` | string | ✅ 实测可用 |
| `images` | array / string | ✅ 实测可用，如 `["http://..."]` |
| `image_urls` | array | 多图 |
| `image` | any | 单图 |
| `mask` | any | 遮罩 |

只要任一字段非空，即判定为"带参考图"（`imageRequestHasReferences()`）。

### 6.2 通路 B：multipart 上传（参考图是本地文件）

```bash
curl -s -X POST "http://domiex-llm-x4xe86-new-api-1:3000/v1/images/edits" \
  -H "Authorization: Bearer $KEY" \
  -F "model=gpt-image-2" \
  -F "prompt=make the apple blue, same composition" \
  -F "image=@/tmp/ref.png" \
  -F "async=true"
```

**真实响应（注意 `kind` 变成 `image_edit`）：**

```json
{
  "created": 1789899128,
  "id": "task_xEE2YaAA1QefbxsRfGMYv1OG9pdsFZTN",
  "kind": "image_edit",
  "object": "task",
  "status": "queued",
  "taskId": "task_xEE2YaAA1QefbxsRfGMYv1OG9pdsFZTN",
  "task_url": "/v1/images/edits/task_xEE2YaAA1QefbxsRfGMYv1OG9pdsFZTN"
}
```

**最终结果**（13 次轮询，180 秒）：

```json
{
  "progress": "100%",
  "result": {
    "finished_at": 1789899308,
    "image_url": "/outputs/task_5ede9186-0684-459e-a2bc-2e448baed8a2.png",
    "model": "gpt-image-2",
    "prompt": "make the apple blue, same composition",
    "status": "completed",
    "task_id": "task_5ede9186-0684-459e-a2bc-2e448baed8a2"
  },
  "status": "succeeded",
  "task_url": "/v1/images/edits/task_xEE2YaAA1QefbxsRfGMYv1OG9pdsFZTN"
}
```

**✅ 视觉验证**：输出苹果为蓝色，构图与参考图一致。
输出 **3840×2160**。

> multipart 请求体会被 new-api 重新组装（`imageAsyncRequestBody()`），
> 原始字节流不影响上游。

### 6.3 ⚠️ multipart 通路不稳定（重要）

通路 B（`/v1/images/edits` 上传本地文件）**实测成功率明显低于通路 A**。同一张参考图、同样简单的改色提示词，三次尝试：

| 通路 | 尝试 | 结果 | 耗时 / 失败原因 |
|---|---|---|---|
| **A** JSON+url | 第 1 次 | ✅ succeeded | 109s → `task_dc097b36-...png` |
| **A** JSON+url | 第 2 次 | ❌ failed | `upstream image service temporarily unavailable, please retry` |
| **A** JSON+url | 第 3 次 | ✅ succeeded | 195s → `task_455f244a-...png` |
| **B** multipart | 第 1 次 | ✅ succeeded | 180s → `task_5ede9186-...png` |
| **B** multipart | 第 2 次 | ❌ failed | ~480s 后 `generate image: 图片生成失败：未返回图片地址` |
| **B** multipart | 第 3 次 | ❌ failed | ~490s 后 `未找到提供的素材，请重新上传素材后重试` |

**统计对比：**

| 通路 | 成功/尝试 | 成功率 | 平均耗时 |
|---|---|---|---|
| A（JSON + image_url） | 2/3 | 67% | ~152s |
| B（multipart 上传） | 1/3 | **33%** | ~183s（失败要等 8 分钟） |

**两个不同的失败含义：**

1. `未返回图片地址` —— 上游跑完但没产出图，属瞬时故障，**重试即可**
2. `未找到提供的素材，请重新上传素材后重试` —— **上传的参考图在上游是临时素材，会失效**。
   任务排队几分钟后素材已被清理，上游就找不到图了。这类失败**重试同一次上传也没用**，必须重新上传。

**结论与建议：**

- ✅ **优先用通路 A（JSON + `image_url`）**——参考图是稳定 URL，不依赖上传的临时素材
- ⚠️ 若手上只有本地文件：**先传到自己的对象存储拿到 URL，再走通路 A**，不要用 multipart
- ⚠️ 若必须走通路 B：接受约 2/3 的失败率，且失败时**要重新上传文件再提交**，不能只重试轮询
- 两条通路的输出都正确（视觉核对：苹果成功变绿 / 变蓝），差异只在可靠性，不在效果

---

## 7. 轮询与结果获取

### 7.1 状态机

| `status`（对外） | 内部 `TaskStatus` | 含义 |
|---|---|---|
| `queued` | SUBMITTED / QUEUED / NOT_START | 已入队 |
| `running` | IN_PROGRESS | 生成中 |
| `succeeded` | SUCCESS | 完成，`result` / `data` 里有 `image_url` |
| `failed` | FAILURE | 失败，看 `error` 字段 |

### 7.2 `progress` 字段

字符串百分比。**实测节奏不线性**：`10% → 30% → 30% → ... → 100%`。
多数时间停在 `30%`，然后直接跳 `100%`。**不要用 progress 做进度条估算真实剩余时间。**

### 7.3 推荐轮询参数

- 间隔：**10–15 秒**（历史前端实际用 8–10 秒）
- 上限：**最坏 707 秒**，建议至少 900 秒超时
- 轮询本身很快（1–3 ms，命中缓存），开销可忽略

### 7.4 完整轮询脚本（bash + jq）

```bash
#!/usr/bin/env bash
set -euo pipefail
BASE="http://domiex-llm-x4xe86-new-api-1:3000"
KEY="$1"
MODEL="${2:-gpt-image-2}"
PROMPT="${3:-a red apple on a wooden table}"

# 1) 提交
SUBMIT=$(curl -s -X POST "$BASE/v1/images/generations" \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d "{\"model\":\"$MODEL\",\"prompt\":\"$PROMPT\",\"n\":1,\"async\":true}")

TASK_URL=$(echo "$SUBMIT" | jq -r '.task_url')
TASK_ID=$(echo "$SUBMIT"  | jq -r '.task_id')
echo "submitted: $TASK_ID"

# 2) 轮询
for i in $(seq 1 90); do
  R=$(curl -s "$BASE$TASK_URL" -H "Authorization: Bearer $KEY")
  S=$(echo "$R" | jq -r '.status')
  echo "  poll $i: $S $(echo "$R" | jq -r '.progress')"
  case "$S" in
    succeeded)
      echo "$R" | jq -r '.result.image_url'
      exit 0
      ;;
    failed)
      echo "FAILED: $(echo "$R" | jq -r '.error')" >&2
      exit 1
      ;;
  esac
  sleep 15
done
echo "timeout" >&2
exit 2
```

### 7.5 用 /outputs/ 直取图片

```bash
IMG="/outputs/task_8e3b19ca-dd26-4e41-a3c1-46ec5d0573ba.png"
curl -s -o out.png "http://domiex-llm-x4xe86-new-api-1:3000$IMG"
# 或直连上游
curl -s -o out.png "http://domiex-air-yw9hej-air2api-1:38474$IMG"
```

---

## 8. 参数速查表

### 请求参数（`dto.ImageRequest`）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `model` | string | ✅ | 见 [模型表](#3-可用模型) |
| `prompt` | string | ✅ | `binding:"required"` |
| `n` | uint | | 生成数量 |
| `size` | string | | 如 `1024x1024`。⚠️ 输出实测不受其限制（见[已知问题](#12-已知问题)） |
| `quality` | string | | 透传上游 |
| `aspect_ratio` | string | | 仅记录进任务 data，未观察到改变输出 |
| `async` | bool | | 触发异步（**实际不传也异步**） |
| `async_task` | bool | | 同上 |
| `return_task_id` | bool | | 同上 |
| `response_format` | string | | ⚠️ **被忽略**，不会返回 b64 |
| `image_url` / `image` | string / any | 图生图 | 参考图 |
| `images` / `image_urls` | array | 图生图 | 多张参考图 |
| `mask` | any | | 遮罩 |
| `callback_url` | string | | 回调地址 |
| `watermark` | bool | | 水印 |

### multipart（`/v1/images/edits`）

| 字段 | 说明 |
|---|---|
| `model` | 模型 ID |
| `prompt` | 提示词 |
| `image` | 文件（`-F image=@file.png`） |
| `async` | `true` |
| `mask` | 可选遮罩文件 |

### 计费

| 项 | 值 |
|---|---|
| 单价 | **25000 quota / 次**（所有 6 个模型同价） |
| 计费方式 | 按次（`PerCallBilling`），日志显示"按次计费，跳过差额结算" |
| 预扣 | 提交时预扣（`PreConsumeBilling`） |
| 失败 | 提交前失败会 `Refund`；上游失败是否退款取决于任务状态机 |

---

## 9. 错误码与排错

### 9.1 实测错误响应

**模型不存在 / group 无权限**（HTTP 200，body 内错误）：

```json
{
  "error": {
    "code": "model_not_found",
    "message": "No available channel for model no-such-model under group svip (distributor) (request id: 202609201015589357318438268d9d6Odh3MmNd)",
    "type": "new_api_error"
  }
}
```

**token 无效**：

```json
{
  "error": {
    "code": "",
    "message": "Invalid token (request id: 20260920101559413310148268d9d6nejqWWQU)",
    "type": "new_api_error"
  }
}
```

**查询不存在的任务**：

```json
{"error": "task_not_found"}          // HTTP 404
{"error": "not_image_task"}          // HTTP 400，task 存在但不是 image 平台
```

### 9.2 任务级失败（`status=failed`）

历史失败原因分布（ClickHouse `new_api.tasks`，platform='image'）：

| 失败原因 | 次数 |
|---|---|
| `generate image: 图片生成失败：未返回图片地址` | 13 |
| `任务超时（600分钟）` | 1 |
| `generate image: 未找到提供的素材，请重新上传素材后重试` | 1 |
| `upstream image service temporarily unavailable, please retry` | 1 |

> ⚠️ 注意：`generate image: 未找到提供的素材` 正是 6.3 节 multipart 临时素材失效的表现。
> 用通路 A（`image_url`）不会遇到这个错误。
>
> 「未返回图片地址」在历史数据里占 13/15，是最主要的失败原因，**重试通常能过**。

> `error` 字段经过 `sanitizeImageTaskPublicError()` 清洗，
> 上游 URL / 本地路径 / TLS 细节会被替换为 `[upstream]` / `[file]`。

### 9.3 排错清单

| 现象 | 原因 | 处理 |
|---|---|---|
| `Invalid token` | token 错 / 已删 / 过期 | 用 `status=1` 的 token |
| `model_not_found ... under group X` | token 的 group 不在 channel 52 的 group 列表 | 改 token group 为 `default/vip/svip/vip1/vip2/vip3/vip6` |
| 提交返回 202 但等不到图 | 正常异步行为 | 必须轮询 `task_url` |
| `response_format: b64_json` 无效 | 被忽略 | 用 `image_url` |
| 任务 `failed` + "未返回图片地址" | 上游未产出图 | 重试 |
| 连不上 `:3000` | new-api 未发布端口 | 走 docker network |
| 415 / multipart 解析失败 | Content-Type 不对 | `/v1/images/edits` 必须 `multipart/form-data` |

---

## 10. 完整可运行示例

### 10.1 Python（异步轮询，含两条通路）

```python
#!/usr/bin/env python3
"""new-api 生图客户端。文生图 / 图生图（URL 与本地文件两种）。"""
import json, time, urllib.request, urllib.error, uuid

BASE = "http://domiex-llm-x4xe86-new-api-1:3000"
KEY  = "QK8PS2TB0U3TTUZQguI7cMbrOGflch7JFoS3eYtOLG8aHhif"
POLL_INTERVAL, POLL_MAX = 15, 90


def _post(url, data, ctype="application/json", headers=None):
    body = json.dumps(data).encode() if ctype == "application/json" else data
    h = {"Authorization": f"Bearer {KEY}", "Content-Type": ctype}
    h.update(headers or {})
    req = urllib.request.Request(url, data=body, headers=h, method="POST")
    with urllib.request.urlopen(req, timeout=120) as r:
        return r.status, json.loads(r.read())


def _get(url):
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {KEY}"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read())


def submit_t2i(model, prompt, size="1024x1024"):
    """文生图。返回 task_id / task_url。"""
    st, body = _post(f"{BASE}/v1/images/generations",
                     {"model": model, "prompt": prompt, "n": 1,
                      "size": size, "async": True})
    assert st == 202, f"expected 202, got {st}: {body}"
    return body["task_id"], body["task_url"]


def submit_i2i_url(model, prompt, image_url):
    """图生图（JSON + 参考图 URL）。"""
    st, body = _post(f"{BASE}/v1/images/generations",
                     {"model": model, "prompt": prompt, "n": 1,
                      "image_url": image_url, "async": True})
    assert st == 202, f"expected 202, got {st}: {body}"
    return body["task_id"], body["task_url"]


def submit_i2i_file(model, prompt, path):
    """图生图（multipart 上传本地文件）。"""
    boundary = uuid.uuid4().hex
    parts = []
    for k, v in (("model", model), ("prompt", prompt), ("async", "true")):
        parts.append(f"--{boundary}\r\nContent-Disposition: form-data; "
                     f'name="{k}"\r\n\r\n{v}\r\n'.encode())
    with open(path, "rb") as fh:
        blob = fh.read()
    parts.append(f"--{boundary}\r\nContent-Disposition: form-data; "
                 f'name="image"; filename="{path.split("/")[-1]}"\r\n'
                 f"Content-Type: image/png\r\n\r\n".encode() + blob + b"\r\n")
    parts.append(f"--{boundary}--\r\n".encode())
    st, body = _post(f"{BASE}/v1/images/edits", b"".join(parts),
                     ctype=f"multipart/form-data; boundary={boundary}")
    assert st == 202, f"expected 202, got {st}: {body}"
    return body["task_id"], body["task_url"]


def poll(task_url, verbose=True):
    """轮询到终态。返回 (status, result_dict)。"""
    for i in range(1, POLL_MAX + 1):
        d = _get(f"{BASE}{task_url}")
        st = d.get("status")
        if verbose:
            print(f"  poll {i}: {st} {d.get('progress', '')}")
        if st == "succeeded":
            return st, d
        if st == "failed":
            return st, d
        time.sleep(POLL_INTERVAL)
    return "timeout", {}


def fetch_image(image_url, out_path):
    """下载 /outputs/xxx.png 到本地。"""
    req = urllib.request.Request(f"{BASE}{image_url}",
                                 headers={"Authorization": f"Bearer {KEY}"})
    with urllib.request.urlopen(req, timeout=300) as r, open(out_path, "wb") as f:
        f.write(r.read())
    return out_path


if __name__ == "__main__":
    # ---- 文生图 ----
    print("== text-to-image ==")
    tid, turl = submit_t2i("gpt-image-2", "a red apple on a wooden table, studio light")
    print("task:", tid)
    st, d = poll(turl)
    if st == "succeeded":
        iu = d["result"]["image_url"]
        print("image_url:", iu)
        fetch_image(iu, "t2i.png")
        print("saved -> t2i.png")

    # ---- 图生图（URL）----
    print("\n== image-to-image (url) ==")
    tid, turl = submit_i2i_url("gpt-image-2",
                              "turn the apple green, keep everything else identical",
                              "http://domiex-air-yw9hej-air2api-1:38474" + iu)
    print("task:", tid)
    st, d = poll(turl)
    if st == "succeeded":
        print("image_url:", d["result"]["image_url"])

    # ---- 图生图（本地文件）----
    print("\n== image-to-image (multipart) ==")
    tid, turl = submit_i2i_file("gpt-image-2", "make the apple blue", "t2i.png")
    print("task:", tid)
    st, d = poll(turl)
    if st == "succeeded":
        print("image_url:", d["result"]["image_url"])
```

### 10.2 一次性验证（curl，复制即用）

```bash
KEY="QK8PS2TB0U3TTUZQguI7cMbrOGflch7JFoS3eYtOLG8aHhif"
BASE="http://domiex-llm-x4xe86-new-api-1:3000"

# 提交
TASK=$(curl -s -X POST "$BASE/v1/images/generations" \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"gpt-image-2","prompt":"a red apple","async":true}')
echo "$TASK" | jq .

# 轮询（每 15s，最多 60 次）
TID=$(echo "$TASK" | jq -r .task_id)
for i in $(seq 1 60); do
  R=$(curl -s "$BASE/v1/images/generations/$TID" -H "Authorization: Bearer $KEY")
  echo "$i: $(echo "$R" | jq -r '.status') $(echo "$R" | jq -r '.progress')"
  [ "$(echo "$R" | jq -r .status)" = "succeeded" ] && echo "$R" | jq -r .result.image_url && break
  sleep 15
done
```

---

## 11. 历史数据与性能基线

数据源：ClickHouse `new_api.tasks`（`platform='image'`）。

### 11.1 任务量（累计）

| action | SUCCESS | FAILURE |
|---|---|---|
| `generate_image`（文生图） | 147 | 14 |
| `edit_image`（图生图） | 100 | 0 |
| `generate_image_video` | 5 | 1 |
| **合计** | **252** | **15** |

成功率约 **94.4%**。

### 11.2 耗时（秒，`finish_time - submit_time`）

| action | 样本 | 均值 | 最小 | 最大 |
|---|---|---|---|---|
| `generate_image` | 153 | **140** | 45 | 707 |
| `edit_image` | 101 | **124** | 54 | 394 |
| `generate_image_video` | 5 | 105 | 70 | 153 |

### 11.3 模型使用分布

| 模型 | 文生图 | 图生图 |
|---|---|---|
| `nano-banana-pro` | 86 | 35 |
| `nano-banana-2` | 42 | 25 |
| `gpt-image-2.5-sunburst` | 12 | **40** |
| `gpt-image-2` | 21 | 1 |
| `gpt-image-2.5-flare` | 5 | 0 |
| `nano-banana-2-lite` | 1 | 0 |

> 图生图历史上最常用 `gpt-image-2.5-sunburst`；文生图最常用 `nano-banana-pro`。

### 11.4 实际输出尺寸（部分高于请求 size）

| 任务 | 输出尺寸 | 大小 |
|---|---|---|
| 文生图 `8e3b19ca` | 3840×3840 | 15.3 MB |
| 图生图(JSON) `dc097b36` | 3840×2146 | 9.0 MB |
| 图生图(multipart) `5ede9186` | 3840×2160 | 8.6 MB |
| 文生图(宿主机复跑) `0875f8c7` | — | — |
| 图生图(JSON) 复跑 `455f244a` | 3840×3840 | 10.4 MB |

> 视觉核对结论（对每张输出图做图像识别确认）：
> - `8e3b19ca`：红苹果 + 木桌 + 棚拍光，1:1，正确
> - `dc097b36` / `455f244a`：**苹果变绿**，构图/光影/木纹与参考图一致 → 图生图生效
> - `5ede9186`：**苹果变蓝**，构图一致 → 图生图生效

---

## 12. 已知问题

1. **`size` 不生效（或至少不严格）**
   实测请求 `size: "1024x1024"`，输出是 **3840×3840**。
   上游（air2api）似乎按自身策略决定分辨率。
   **不要依赖 `size` 控制输出尺寸**，拿到图后按需自行缩放。

2. **`response_format` 被忽略**
   `b64_json` 不会生效，永远返回 task_id + `image_url`。

3. **`progress` 非线性**
   长时间停在 `30%`，然后直接跳 `100%`。不能用来估算剩余时间。

4. **`image_url` 是相对路径**
   `/outputs/xxx.png`，需要拼接 base URL。且这是容器内路径 —— 宿主机未发布 new-api 端口，外部网络无法直接访问，需要反代或走 docker network。

5. **最长耗时 707 秒（近 12 分钟）**
   轮询超时必须给够，建议 ≥900 秒。

6. **`async` 参数形同虚设**
   不传也是异步。客户端不能假设不传 `async` 就能拿到同步图片。

7. **`task_url` 路径与实际 kind 可能不一致**
   图生图走 JSON 通路时 `kind=image_generation`、`task_url` 指向 `/generations`，
   但内容是图生图。**以 `task_url` 为准轮询，不要用 `kind` 判断。**

---

## 附：相关源码位置

| 功能 | 文件 |
|---|---|
| 异步生图主流程 | `controller/image_async.go` |
| 请求 DTO | `dto/openai_image.go` |
| 路由注册 | `router/relay-router.go:87,135` |
| 任务模型 | `model/task.go` |
| 定价 | `model/option.go:30,276-281` |
| 上游分发（air2api / felo） | `controller/image_async.go:shouldRouteImageRequestToFelo()` |

## 附：基础设施速查

| 组件 | 地址 | 说明 |
|---|---|---|
| new-api | `domiex-llm-x4xe86-new-api-1:3000` | 容器内；宿主未发布 |
| 生图上游 | `domiex-air-yw9hej-air2api-1:38474` | air2api，channel 52 后端 |
| ClickHouse | `domiex-clickhouse-5evaye-clickhouse-1:9000` | `new_api` / `new_api_logs` 库 |
| Redis | `domiex-llm-x4xe86-redis-1:6379` | 任务轮询缓存 |

查询任务历史：

```bash
docker exec domiex-clickhouse-5evaye-clickhouse-1 clickhouse-client \
  --user default --password <pw> -d new_api -q "
select task_id, action, status, fail_reason,
       finish_time-submit_time as secs,
       JSONExtractString(properties,'upstream_model_name') as model
from tasks where platform='image'
order by id desc limit 20"
```
