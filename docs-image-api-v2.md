# new-api 生图接口文档（文生图 / 图生图）

> 计费：**25000 quota / 次**，按次计费，与模型无关
> 任务超时：600 分钟
> 上游渠道：channel 52「Image Omni」（OpenAI 兼容类型）
>
> 本文所有请求/响应均为**实跑结果**，非推测。文中 `task_xxx` 是真实任务 ID。

---

## 目录

1. [核心结论](#1-核心结论)
2. [鉴权](#2-鉴权)
3. [可用模型](#3-可用模型)
4. [接口总览](#4-接口总览)
5. [文生图](#5-文生图)
6. [图生图](#6-图生图)
7. [轮询与取图](#7-轮询与取图)
8. [参数速查表](#8-参数速查表)
9. [错误与排错](#9-错误与排错)
10. [完整示例](#10-完整示例)
11. [性能基线](#11-性能基线)
12. [已知问题](#12-已知问题)

---

## 1. 核心结论

**所有生图请求都是异步的，且强制异步。**

- `POST /v1/images/generations` 或 `POST /v1/images/edits` **立即返回 HTTP 202**，body 是 `task` 对象，**不含图片**。
- 即使**不传** `async` 参数，也一样返回 202 + task（实测 46ms 返回）。
- **`response_format: "b64_json"` 被忽略**，仍返回 task_id。取图必须轮询。
- 实测耗时 **45–707 秒**（均值 124–140 秒），轮询超时要给够。

触发异步的显式参数（三者任一即可，但实际不传也是异步）：`async`、`async_task`、`return_task_id`。

---

## 2. 鉴权

```http
Authorization: Bearer <token>
Content-Type: application/json
```

- token 是 new-api 的 token（48 字符），不是上游 key
- 模型能否路由取决于 **token 所属 group**。生图渠道挂在
  `default, vip, svip, vip1, vip2, vip3, vip6`，token 的 group 必须落在其中，
  否则报 `model_not_found`

---

## 3. 可用模型

6 个，**全部同价 25000 quota/次**。

| 模型 ID | 文生图 | 图生图 | 备注 |
|---|---|---|---|
| `gpt-image-2` | ✅ | ✅ | 通用 |
| `gpt-image-2.5-sunburst` | ✅ | ✅ | 图生图历史使用最多 |
| `gpt-image-2.5-flare` | ✅ | — | — |
| `nano-banana-2` | ✅ | ✅ | 快速、多比例 |
| `nano-banana-2-lite` | ✅ | — | 最便宜档 |
| `nano-banana-pro` | ✅ | ✅ | 历史使用最多，支持 4K |

---

## 4. 接口总览

| 方法 | 路径 | 作用 | 返回 |
|---|---|---|---|
| POST | `/v1/images/generations` | 提交任务（文生图 / 图生图 JSON） | **202** |
| POST | `/v1/images/edits` | 提交任务（图生图 multipart 上传） | **202** |
| GET | `/v1/images/generations/{task_id}` | 查询任务 | 200 |
| GET | `/v1/images/edits/{task_id}` | 查询任务 | 200 |

**用提交时返回的 `task_url` 轮询**。提交响应里 `id` / `task_id` / `taskId`
三个字段值相同，冗余兼容不同客户端。

---

## 5. 文生图

### 5.1 请求

```json
{
  "model": "nano-banana-2",
  "prompt": "你的提示词",
  "aspect_ratio": "9:16",
  "resolution": "2K"
}
```

### 5.2 响应（HTTP 202）

```json
{
  "created": 1789901445,
  "id": "task_w04WzbqLPtSQL6uDgbZOsW6GMmLtBTpn",
  "kind": "image_generation",
  "object": "task",
  "status": "queued",
  "taskId": "task_w04WzbqLPtSQL6uDgbZOsW6GMmLtBTpn",
  "task_id": "task_w04WzbqLPtSQL6uDgbZOsW6GMmLtBTpn",
  "task_url": "/v1/images/generations/task_w04WzbqLPtSQL6uDgbZOsW6GMmLtBTpn",
  "updated": 1789901445
}
```

### 5.3 最终结果

```json
{
  "created": 1789901445,
  "kind": "image_generation",
  "object": "task",
  "progress": "100%",
  "result": {
    "created_at": 1789901445,
    "finished_at": 1789901601,
    "id": "cbb30acd-...",
    "image_url": "/outputs/task_cbb30acd-....png",
    "model": "nano-banana-2",
    "progress": 100,
    "prompt": "a lighthouse at sunset",
    "status": "completed",
    "task_id": "task_cbb30acd-....",
    "type": "image"
  },
  "status": "succeeded",
  "task_id": "task_w04WzbqLPtSQL6uDgbZOsW6GMmLtBTpn",
  "task_url": "/v1/images/generations/task_w04WzbqLPtSQL6uDgbZOsW6GMmLtBTpn"
}
```

**取图**：`result.image_url` 是相对路径 `/outputs/xxx.png`。

> ⚠️ **这个路径不属于 API 服务。**
> 把它拼到 API 的 base 上，会返回 **`200 text/html`** —— 那是前端 SPA 的兜底页，**不是图片**（HTTP 200 具有欺骗性，必须检查 `Content-Type`）。
> 图片由**上游图片服务**提供，要用图片服务的 base 去拼：
>
> ```
> <图片服务地址>/outputs/task_<uuid>.png      → 200 image/png
> ```
>
> 也就是说 **API base ≠ 图片 base**，两者是两个不同的服务。详见 [7.5](#75-下载图片)。

---

## 6. 图生图

用 **`reference_images` 数组**传参考图（不是 `image_url` / `images`）。

### 6.1 单图参考

```json
{
  "model": "nano-banana-pro",
  "prompt": "把苹果改成绿色，其余保持不变",
  "aspect_ratio": "16:9",
  "resolution": "2K",
  "reference_images": ["https://cdn.example.com/ref.png"]
}
```

### 6.2 多图参考（海报合成）

```json
{
  "model": "nano-banana-pro",
  "prompt": "【图片1】是场景，【图片2】是主角，【图片3】是配角，【图片4】是色调参考，合成一张电影海报",
  "aspect_ratio": "16:9",
  "resolution": "2K",
  "reference_images": [
    "https://cdn.example.com/bg.jpg",
    "https://cdn.example.com/hero.png",
    "https://cdn.example.com/support.png",
    "https://cdn.example.com/color-ref.png"
  ]
}
```

**实测这条成功**（用 3 张已有输出的 `/outputs/` 地址做参考图）：

```json
{
  "created": 1789901445,
  "id": "task_ffDWFnzawVCfalCVhsuRn8S545NxC2AR",
  "kind": "image_generation",
  "object": "task",
  "status": "queued",
  "task_url": "/v1/images/generations/task_ffDWFnzawVCfalCVhsuRn8S545NxC2AR"
}
```

结果：`status: "succeeded"`，输出 **2752×1536**（16:9），
内容为合成海报（含标题与 credit 区块），**多图参考生效**。

### 6.3 参考图字段的兼容写法

以下是全部被接受的字段，任一非空即判定为「带参考图」：

| 字段 | 状态 |
|---|---|
| **`reference_images`**（数组） | ✅ **推荐 / 本文使用** |
| `images` | ✅ 兼容 |
| `image_url` | ✅ 兼容 |
| `image_urls` | ✅ 兼容 |
| `image` | ✅ 兼容 |

**多图参考请用 `reference_images`。**

### 6.4 multipart 上传通路（不推荐）

`POST /v1/images/edits` 支持 `multipart/form-data` 上传本地文件：

| 字段 | 说明 |
|---|---|
| `model` | 模型 ID |
| `prompt` | 提示词 |
| `image` | 文件 |
| `aspect_ratio` / `resolution` | 同上 |

**⚠️ 实测可靠性差**（同一张参考图、同样简单的改色提示词，3 次尝试）：

| 尝试 | 结果 | 耗时 / 失败原因 |
|---|---|---|
| 第 1 次 | ✅ succeeded | 180s → `/outputs/task_5ede9186-....png` |
| 第 2 次 | ❌ failed | ~480s 后 `generate image: 图片生成失败：未返回图片地址` |
| 第 3 次 | ❌ failed | ~490s 后 `未找到提供的素材，请重新上传素材后重试` |

**成功率仅 1/3。**

两个失败的**含义不同**：

1. `未返回图片地址` —— 上游跑完没产出图，瞬时故障，**重试即可**
2. `未找到提供的素材，请重新上传素材后重试` —— **上传的参考图是上游临时素材，排队几分钟后被清理**。
   这类失败重试同一次上传没用，**必须重新上传文件**

**结论：**
- ✅ **优先用 JSON + `reference_images`（URL）**
- ⚠️ 只有本地文件时：**先传到自己的存储拿到 URL**，再走 JSON 通路
- ⚠️ 必须走 multipart 时：接受约 2/3 失败率，失败后要**重新上传再提交**

两条通路的输出都正确（视觉核对：苹果成功变蓝），差异只在可靠性。

---

## 7. 轮询与取图

### 7.1 状态机

| `status` | 含义 |
|---|---|
| `queued` | 已入队 |
| `running` | 生成中 |
| `succeeded` | 完成，`result.image_url` 里有图 |
| `failed` | 失败，看 `error` 字段 |

### 7.2 `progress`

字符串百分比。**非线性**：`10% → 30% → 30% … → 100%`。
多数时间停在 `30%`，然后直接跳 `100%`。**不能用来估剩余时间。**

### 7.3 推荐轮询参数

- 间隔：**10–15 秒**
- 上限：最坏 **707 秒**，建议超时 ≥900 秒
- 轮询很轻（1–3 ms，命中缓存）

### 7.4 轮询脚本

```bash
#!/usr/bin/env bash
set -euo pipefail
BASE="$1"      # 服务地址
KEY="$2"
MODEL="${3:-nano-banana-2}"
PROMPT="${4:-a lighthouse at sunset}"

SUBMIT=$(curl -s -X POST "$BASE/v1/images/generations" \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d "{\"model\":\"$MODEL\",\"prompt\":\"$PROMPT\",\"aspect_ratio\":\"9:16\",\"resolution\":\"2K\"}")

TASK_ID=$(echo "$SUBMIT" | jq -r '.task_id')
TASK_URL=$(echo "$SUBMIT" | jq -r '.task_url')
echo "submitted: $TASK_ID"

for i in $(seq 1 90); do
  R=$(curl -s "$BASE$TASK_URL" -H "Authorization: Bearer $KEY")
  S=$(echo "$R" | jq -r '.status')
  echo "  poll $i: $S $(echo "$R" | jq -r '.progress')"
  case "$S" in
    succeeded) echo "$R" | jq -r '.result.image_url'; exit 0 ;;
    failed)    echo "FAILED: $(echo "$R" | jq -r '.error')" >&2; exit 1 ;;
  esac
  sleep 15
done
echo "timeout" >&2
exit 2
```

### 7.5 下载图片（⚠️ 最容易踩的坑）

`result.image_url` = `/outputs/task_<uuid>.png` 是**上游图片服务**的路径，
**不是 API 服务的路径**。

| 拼接方式 | 结果 |
|---|---|
| `<API base>/outputs/task_x.png` | ❌ **200 但是 `text/html`**（前端兜底页，不是图） |
| `<图片服务 base>/outputs/task_x.png` | ✅ **200 `image/png`** |

**必须用图片服务的 base。** 两个 base 可能相同也可能不同，取决于部署方式，
但**一定要用 `Content-Type` 校验，不能只看状态码**：

```bash
IMG_URL="/outputs/task_<uuid>.png"

# 正确：拼图片服务的 base
curl -s -o out.png -w "%{http_code} %{content_type}\n" \
  "${IMAGE_BASE}${IMG_URL}"

# 校验（关键：不要只看 200）
file out.png          # 应为 "PNG image data" / "JPEG image data"
# 或
curl -sI "${IMAGE_BASE}${IMG_URL}" | grep -i content-type
```

正确下载示例：

```bash
IMAGE_BASE="http://<图片服务地址>"     # 例如 http://<your-image-host>:<port>
curl -s -o out.png "$IMAGE_BASE/outputs/task_2556ee35-....png"
file out.png    # → PNG image data, 1536 x 2752
```

> ⚠️ **任务 `succeeded` 不等于图片立即可下载。**
> 实测多次出现：轮询刚返回 `succeeded`，立刻去下载却拿到 **404**，
> 稍后重试同一 URL 就变成 200。
> 所以下载逻辑**必须带重试**（建议 3s 间隔、10 次），
> 不要用「轮询成功 → 单次下载」的写法，否则会随机丢图。
> 本文 Python 示例的 `download()` 已经内置重试。

---

## 8. 参数速查表

### 请求参数

| 参数 | 类型 | 说明 |
|---|---|---|
| `model` | string | 必填，见[模型表](#3-可用模型) |
| `prompt` | string | 必填 |
| `aspect_ratio` | string | 画面比例：`16:9` / `9:16` / `1:1` / `3:4` / `21:9` … |
| `resolution` | string | 分辨率档位：`1K` / `2K` / `4K` |
| `reference_images` | array | 参考图 URL 数组（图生图，支持多图） |
| `n` | int | 生成数量 |
| `size` | string | 兼容字段；**优先用 `aspect_ratio` + `resolution`** |
| `quality` | string | 透传上游 |
| `async` / `async_task` / `return_task_id` | bool | 触发异步（不传也异步） |
| `response_format` | string | ⚠️ 被忽略 |
| `callback_url` | string | 回调地址 |
| `watermark` | bool | 水印 |

### 实测比例 → 输出尺寸

| 请求 | 输出尺寸 |
|---|---|
| `9:16` + `2K` | 1536 × 2752 |
| `16:9` + `2K` | 2752 × 1536 |
| `1:1` + `2K` | 2048 × 2048 |
| `3:4` + `4K` | 2867 × 3840 |
| `21:9` + `2K` | 3168 × 1344 |

**比例和分辨率真实生效**（见[已知问题](#12-已知问题)第 2 条关于非法值的处理）。

### 计费

| 项 | 值 |
|---|---|
| 单价 | **25000 quota / 次**（6 个模型同价） |
| 方式 | 按次计费，不按 token |
| 时点 | 提交时预扣 |

---

## 9. 错误与排错

### 9.1 提交阶段错误

**模型不存在 / group 无权限**（HTTP 200，body 内错误）：

```json
{
  "error": {
    "code": "model_not_found",
    "message": "No available channel for model no-such-model under group svip (distributor) (request id: ...)",
    "type": "new_api_error"
  }
}
```

**token 无效**：

```json
{
  "error": {
    "code": "",
    "message": "Invalid token (request id: ...)",
    "type": "new_api_error"
  }
}
```

**查询不存在的任务**：

```json
{"error": "task_not_found"}   // HTTP 404
{"error": "not_image_task"}   // HTTP 400
```

### 9.2 任务级失败（`status=failed`）

历史失败原因分布：

| 失败原因 | 次数 |
|---|---|
| `generate image: 图片生成失败：未返回图片地址` | 13 |
| `任务超时（600分钟）` | 1 |
| `generate image: 未找到提供的素材，请重新上传素材后重试` | 1 |
| `upstream image service temporarily unavailable, please retry` | 1 |

- 「未返回图片地址」占绝大多数，**重试通常能过**
- 「未找到提供的素材」是 multipart 临时素材失效（见 6.4）
- `error` 字段经清洗，上游 URL / 本地路径 / TLS 细节会被替换为 `[upstream]` / `[file]`

### 9.3 排错清单

| 现象 | 原因 | 处理 |
|---|---|---|
| `Invalid token` | token 错 / 已删 / 过期 | 换有效 token |
| `model_not_found ... under group X` | token 的 group 不在渠道列表 | 改为 `default/vip/svip/vip1/vip2/vip3/vip6` |
| 返回 202 但拿不到图 | 正常异步 | 必须轮询 `task_url` |
| `response_format: b64_json` 无效 | 被忽略 | 用 `image_url` |
| 任务 `failed` + 「未返回图片地址」 | 上游未产出图 | 重试 |
| 任务 `failed` + 「未找到提供的素材」 | multipart 临时素材失效 | 重新上传，或改用 URL |
| 轮询超时 | 上游耗时最长近 12 分钟 | 超时设 ≥900 秒 |
| 415 / multipart 解析失败 | Content-Type 不对 | `/v1/images/edits` 必须 `multipart/form-data` |

---

## 10. 完整示例

### 10.1 Python（文生图 + 图生图，含轮询）

```python
#!/usr/bin/env python3
"""生图客户端：文生图 / 图生图（含多图参考）。"""
import json, time, urllib.error, urllib.request

BASE = "http://<API 服务地址>"       # 提交/轮询用
IMAGE_BASE = "http://<图片服务地址>"  # 下载图片用（可能与 BASE 不同，见 7.5）
KEY  = "<你的 token>"
POLL_INTERVAL, POLL_MAX = 15, 90


def _req(path, data=None):
    url = f"{BASE}{path}"
    hdr = {"Authorization": f"Bearer {KEY}"}
    if data is None:
        req = urllib.request.Request(url, headers=hdr)
    else:
        hdr["Content-Type"] = "application/json"
        req = urllib.request.Request(url, data=json.dumps(data).encode(),
                                     headers=hdr, method="POST")
    with urllib.request.urlopen(req, timeout=120) as r:
        return r.status, json.loads(r.read())


def submit(model, prompt, aspect_ratio="16:9", resolution="2K",
           reference_images=None):
    """提交任务，返回 (task_id, task_url)。"""
    payload = {"model": model, "prompt": prompt,
               "aspect_ratio": aspect_ratio, "resolution": resolution}
    if reference_images:
        payload["reference_images"] = reference_images
    st, body = _req("/v1/images/generations", payload)
    assert st == 202, f"expected 202, got {st}: {body}"
    return body["task_id"], body["task_url"]


def poll(task_url, label="", verbose=True):
    """轮询到终态，返回响应 dict。"""
    for i in range(1, POLL_MAX + 1):
        _, d = _req(task_url)
        st = d.get("status")
        if verbose:
            print(f"  [{label}] poll {i}: {st} {d.get('progress', '')}")
        if st in ("succeeded", "failed"):
            return d
        time.sleep(POLL_INTERVAL)
    return {"status": "timeout"}


def download(image_url, out_path, retries=10, delay=3):
    """下载 /outputs/xxx.png 到本地。

    注意两点：
    1. image_url 属于图片服务，不是 API 服务 → 用 IMAGE_BASE
    2. 任务刚 succeeded 时图片可能还没就绪（实测有数秒延迟），
       所以这里带重试，并且校验 Content-Type
    """
    last_err = None
    for _ in range(retries):
        try:
            req = urllib.request.Request(f"{IMAGE_BASE}{image_url}",
                                         headers={"Authorization": f"Bearer {KEY}"})
            with urllib.request.urlopen(req, timeout=300) as r:
                ctype = r.headers.get("Content-Type", "")
                blob = r.read()
            # 关键校验：API base 会返回 200 + text/html，那是前端页面不是图
            if not ctype.startswith("image/"):
                raise RuntimeError(
                    f"expected image, got {ctype} — 检查 IMAGE_BASE 是否是图片服务地址")
            with open(out_path, "wb") as f:
                f.write(blob)
            return out_path
        except urllib.error.HTTPError as e:
            last_err = e            # 404 = 图片尚未落盘，重试
            time.sleep(delay)
    raise RuntimeError(f"download failed after {retries} tries: {last_err}")


if __name__ == "__main__":
    # ---- 文生图 ----
    print("== text-to-image ==")
    tid, turl = submit("nano-banana-2", "a lighthouse at sunset",
                       aspect_ratio="9:16", resolution="2K")
    print("task:", tid)
    d = poll(turl, "t2i")
    if d.get("status") == "succeeded":
        iu = d["result"]["image_url"]
        print("image_url:", iu)
        download(iu, "t2i.png")
        print("saved -> t2i.png")

    # ---- 图生图（多图参考）----
    print("\n== image-to-image (multi reference) ==")
    tid, turl = submit(
        "nano-banana-pro",
        "【图片1】是场景，【图片2】是主角，【图片3】是配角，【图片4】是色调参考，合成一张电影海报",
        aspect_ratio="16:9", resolution="2K",
        reference_images=[
            "https://cdn.example.com/bg.jpg",
            "https://cdn.example.com/hero.png",
            "https://cdn.example.com/support.png",
            "https://cdn.example.com/color-ref.png",
        ])
    print("task:", tid)
    d = poll(turl, "i2i")
    if d.get("status") == "succeeded":
        print("image_url:", d["result"]["image_url"])
```

### 10.2 curl 一次性验证

```bash
BASE="http://<服务地址>"
KEY="<你的 token>"

# 文生图
SUBMIT=$(curl -s -X POST "$BASE/v1/images/generations" \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"nano-banana-2","prompt":"a lighthouse at sunset","aspect_ratio":"9:16","resolution":"2K"}')
echo "$SUBMIT" | jq .

# 轮询
TID=$(echo "$SUBMIT" | jq -r .task_id)
for i in $(seq 1 60); do
  R=$(curl -s "$BASE/v1/images/generations/$TID" -H "Authorization: Bearer $KEY")
  echo "$i: $(echo "$R" | jq -r .status) $(echo "$R" | jq -r .progress)"
  [ "$(echo "$R" | jq -r .status)" = "succeeded" ] && \
    echo "$R" | jq -r .result.image_url && break
  sleep 15
done
```

### 10.3 图生图 curl

```bash
BASE="http://<服务地址>"
KEY="<你的 token>"

curl -s -X POST "$BASE/v1/images/generations" \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{
    "model": "nano-banana-pro",
    "prompt": "【图片1】是场景，【图片2】是主角，合成一张电影海报",
    "aspect_ratio": "16:9",
    "resolution": "2K",
    "reference_images": [
      "https://cdn.example.com/bg.jpg",
      "https://cdn.example.com/hero.png"
    ]
  }' | jq .
```

---

## 11. 性能基线

### 11.1 任务量（累计）

| action | SUCCESS | FAILURE |
|---|---|---|
| `generate_image`（文生图） | 147 | 14 |
| `edit_image`（图生图） | 100 | 0 |
| `generate_image_video` | 5 | 1 |
| **合计** | **252** | **15** |

成功率约 **94.4%**。

### 11.2 耗时（秒）

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

### 11.4 实测输出尺寸

| 请求 | 输出尺寸 |
|---|---|
| `9:16` + `2K` | 1536 × 2752 |
| `16:9` + `2K` | 2752 × 1536 |
| `1:1` + `2K` | 2048 × 2048 |
| `3:4` + `4K` | 2867 × 3840 |
| `21:9` + `2K` | 3168 × 1344 |
| 图生图（多图参考 `16:9` + `2K`） | 2752 × 1536（成功合成海报） |
| 图生图（单图参考 `16:9` + `2K`） | 3840 × 3840（苹果改紫成功） |

> 视觉核对（对每张输出做图像识别确认）：
> - 文生图 `9:16`：竖版灯塔日落，构图/光影连贯，方向正确
> - 图生图（单图改色）：苹果成功变绿 / 变蓝，构图与参考图一致
> - 图生图（多图参考）：成功合成海报（含标题与 credit 区块）

---

## 12. 已知问题

1. **`size` 不生效，要用 `aspect_ratio` + `resolution`**
   传 `size: "1024x1024"` 得到 3840×3840；改用 `aspect_ratio` + `resolution`
   后输出精确匹配（见 8 节表格）。
   **用 `aspect_ratio` + `resolution`，不要用 `size`。**

2. **非法参数值不报错，只在任务阶段失败**
   - `aspect_ratio: "7:5"`（不在支持列表）→ 提交返回 **202**，随后任务 `failed`
   - `resolution: "8K"`（超出 `1K/2K/4K`）→ 提交返回 **202**，随后任务 `failed`
   → **必须在客户端自行校验参数**，不能依赖提交时的报错。

3. **`response_format` 被忽略**，永远返回 task_id + `image_url`。

4. **`progress` 非线性**，不能估时。

5. **`image_url` 是相对路径，且属于图片服务**
   `result.image_url` = `/outputs/xxx.png`。拼到 **API base** 上会返回
   **200 `text/html`**（前端兜底页，不是图），必须拼 **图片服务 base**（见 7.5）。
   **校验时一定要看 `Content-Type`，只看状态码会误判成功。**

6. **最长耗时 707 秒**，轮询超时 ≥900 秒。

7. **`async` 形同虚设**，不传也是异步。

8. **multipart 通路可靠率仅约 33%**（见 6.4），优先用 `reference_images` URL。

9. **`task_url` 的 `kind` 可能与实际不符**
   图生图走 JSON 通路时 `kind` 仍报 `image_generation`。
   **以 `task_url` 为准轮询，不要用 `kind` 判断。**

10. **`succeeded` 后图片可能短暂 404**
    轮询拿到 `succeeded` 不代表 `image_url` 立刻可下载，实测会出现 404 随后变 200。
    **下载必须带重试**（见 7.5）。
