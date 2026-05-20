# Lic

基于 Golang + Gin + GORM 实现的软件授权验证服务端，支持在线和离线两种激活方式。

## 功能特性

- **帐密登录**：用户名/密码登录，返回 JWT Token
- **在线激活**：客户端启动时通过 Token + 设备指纹在线激活
- **离线激活**：支持离线环境下的授权激活（请求文件 + 响应文件机制）
- **授权码管理**：创建、查询、撤销、延期、禁用/启用授权码
- **多选产品支持**：一个授权码可同时授权多个产品（一码多用）
- **设备绑定**：授权码与设备指纹绑定，控制激活数量
- **有效期控制**：精确控制授权开始/结束时间
- **RSA 签名**：离线激活响应文件使用 RSA 私钥签名，防止篡改

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 启动服务

```bash
# SQLite（默认）
go run main.go

# MySQL
DB_TYPE=mysql DB_DSN="user:pass@tcp(127.0.0.1:3306)/license?charset=utf8mb4" go run main.go

# 自定义 JWT 密钥和端口
JWT_SECRET=your-secret-key HTTP_ADDR=:9090 go run main.go
```

### 3. 注册用户（自动生成授权码）

```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"123456","email":"test@example.com"}'
```

响应示例：

```json
{
  "message": "registration successful",
  "user_id": 1,
  "license_key": "A1B2-C3D4-E5F6-7890"
}
```

## API 文档

### 公开接口（无需 Token）

| 方法   | 路径                                        | 说明                |
|------|-------------------------------------------|-------------------|
| POST | `/api/v1/register`                        | 注册用户（自动生成授权码）     |
| POST | `/api/v1/login`                           | 帐密登录，返回 JWT Token |
| POST | `/api/v1/verify`                          | 验证授权码状态（无需登录）     |
| GET  | `/api/v1/products`                        | 获取产品列表（供申请页面使用）  |
| POST | `/api/v1/offline/verify`                  | 客户端验证离线响应文件       |
| POST | `/api/v1/offline/request/gen`             | 生成离线激活请求          |
| GET  | `/api/v1/offline/request/:token/download` | 下载离线请求文件          |
| POST | `/api/v1/offline/activate/:token`         | 处理离线激活，生成响应文件     |

### 认证接口（需要 JWT Token，Header: `Authorization: Bearer <token>`）

| 方法   | 路径                   | 说明           |
|------|----------------------|--------------|
| POST | `/api/v1/activate`   | 在线激活（绑定设备指纹） |
| GET  | `/api/v1/license/me` | 查看我的授权信息     |
| GET  | `/api/v1/licenses`   | 列出我的授权码      |

### 管理员接口（需要 JWT Token）

| 方法   | 路径                                                       | 说明      |
|------|----------------------------------------------------------|---------|
| POST | `/api/v1/admin/license`                                  | 创建授权码   |
| GET  | `/api/v1/admin/licenses?all=true`                        | 列出所有授权码 |
| GET  | `/api/v1/admin/license/:licenseKey`                      | 查看授权码详情 |
| PUT  | `/api/v1/admin/license/:licenseKey/disable`              | 禁用授权码   |
| PUT  | `/api/v1/admin/license/:licenseKey/enable`               | 启用授权码   |
| PUT  | `/api/v1/admin/license/:licenseKey/revoke`               | 撤销授权码   |
| PUT  | `/api/v1/admin/license/:licenseKey/extend`               | 延长授权有效期 |
| GET  | `/api/v1/admin/license/:licenseKey/activations`          | 查看激活记录  |
| PUT  | `/api/v1/admin/license/:licenseKey/deactivate/:deviceFP` | 禁用指定设备  |

## 在线激活流程

```
客户端启动
  └─> POST /api/v1/login（帐密）─> 获取 Token
        └─> POST /api/v1/activate（Token + license_key + device_fingerprint）
              └─> 激活成功，返回 valid_to
                    └─> 每次启动：POST /api/v1/verify（license_key）
```

## Web 管理界面

项目提供完整的 Web 管理界面，无需 API 调用即可完成日常操作：

### 管理员界面

- **访问路径**：`/admin`（需要管理员登录）
- **功能**：
  - 查看所有授权码列表（支持按状态筛选）
  - 创建新授权码
  - 禁用/启用授权码（临时停用可恢复）
  - 撤销授权码（永久失效）
  - 延长授权有效期
  - 查看激活记录
  - 审核授权码申请（通过/拒绝）

- **授权码操作规则**：
  - `active`（激活）→ 显示【禁用】【延期】【撤销】按钮
  - `disabled`（已禁用）→ 显示【启用】【撤销】按钮
  - `expired`/`revoked`（已过期/已撤销）→ 仅显示【撤销】按钮

### 用户申请界面

- **访问路径**：`/apply`（公开访问）
- **功能**：
  - 填写申请信息（姓名、邮箱、用途等）
  - 选择产品（支持多选，页面自动从 `/api/v1/products` 加载）
  - 提交申请，等待管理员审核
  - 查看申请状态和历史记录

## 离线激活流程

适用于无法连接外网的客户端环境：

```
[离线客户端]
  1. POST /api/v1/offline/request/gen（license_key + device_fingerprint）
     └─> 获得 request.json 内容
  2. 将 request.json 手动传输到联网机器

[联网机器]
  3. POST /api/v1/offline/activate/<token>
     └─> 下载 response.json（服务端 RSA 签名）
  4. 将 response.json 手动传回离线客户端

[离线客户端]
  5. POST /api/v1/offline/verify（response_file 内容）
     └─> 验证签名 + 有效期 ─> 激活成功
```

## 离线授权验证（客户端离线使用）

适用于客户端无法连接外网，但仍需验证授权有效性的场景。

### 原理

1. 客户端联网时，下载**离线授权文件**（包含授权信息 + RSA签名）
2. 客户端将文件保存到本地
3. 离线时，客户端验证本地授权文件：
   - 验证 RSA 签名（使用服务端公钥）
   - 检查授权有效期（`valid_from` / `valid_to`）
4. **无需定期联网**，文件在授权有效期内始终有效

### 使用流程

```
[客户端联网时]
  1. GET /api/v1/license/:licenseKey/offline-auth-file
     └─> 下载 offline-auth-<licenseKey>.json
  2. 保存文件到本地

[客户端离线时]
  3. 读取本地 offline-auth-<licenseKey>.json
  4. 验证文件签名和有效期
     └─> 验证通过 → 授权有效
     └─> 验证失败 → 授权无效，提示用户联网
```

### API 接口

#### 下载离线授权文件（需要 JWT 认证）

```
GET /api/v1/license/:licenseKey/offline-auth-file
Header: Authorization: Bearer <token>
```

**响应**：返回 JSON 格式的离线授权文件，浏览器自动下载。

**文件内容示例**：
```json
{
  "version": "1.0",
  "license_key": "A1B2-C3D4-E5F6-7890",
  "company": "ABC科技有限公司",
  "product_keys": ["standard", "pro"],
  "valid_from": "2026-01-01T00:00:00Z",
  "valid_to": "2027-01-01T00:00:00Z",
  "activated_count": 2,
  "max_activations": 5,
  "issued_at": "2026-01-01T00:00:00Z",
  "signature": "a1b2c3d4e5f6...",
  "certificate": "-----BEGIN PUBLIC KEY-----\n..."
}
```

### 客户端验证步骤

1. **验证签名**：
   - 使用服务端公钥（`certificate` 字段）验证 `signature` 字段
   - 签名数据：`version|license_key|company|valid_from|valid_to|activated_count|max_activations`

2. **验证授权有效期**：
   - 检查当前时间是否在 `valid_from` 和 `valid_to` 之间

### 客户端示例代码

**Golang SDK**：参见 `cmd/client_example/main.go` 和 `sdk/go/`

```bash
# 1. 下载离线授权文件（需要 JWT Token）
go run cmd/client_example/main.go download <token> <license_key>

# 2. 验证离线授权文件（无需联网）
go run cmd/client_example/main.go verify <license_key>
```

**Python SDK**：参见 `sdk/python/`

```bash
# 安装依赖
cd sdk/python
pip install -r requirements.txt

# 验证授权文件
python3 example.py verify

# 下载授权文件
python3 example.py download
```

Python SDK 提供以下 API：
- `verify_offline_auth_file(file_path, public_key)` - 验证本地授权文件
- `is_license_valid(file_path, public_key)` - 简单检查授权是否有效
- `verify_offline_auth_file_from_data(auth_file, public_key)` - 验证内存中的数据
- `download_offline_auth_file(server_url, token, license_key, save_path)` - 从服务器下载授权文件

详细文档请查看 `sdk/python/README.md`

### 生产环境部署建议

1. **服务端**：
   - 生成 RSA 密钥对（`rsa_private.pem` / `rsa_public.pem`）
   - 私钥用于签名离线授权文件
   - 公钥分发给所有客户端（嵌入客户端程序）

2. **客户端**：
   - 定期（如每24小时）尝试联网下载最新离线授权文件
   - 离线时，使用本地文件验证授权
   - 超过宽限期（如7天）后，强制要求联网

3. **安全建议**：
   - 公钥可以公开，但确保客户端使用的是正确的公钥
   - 建议将公钥硬编码到客户端程序中
   - 定期检查授权文件是否被篡改

## 数据库表结构

- **users**：用户账号（用户名、密码哈希、邮箱）
- **licenses**：授权码（license_key、产品列表JSON、有效期、状态、最大激活数、已激活数）
  - `product_keys`: JSON 数组，支持多个产品（一码多用）
  - `status`: 状态字段，支持 `active`（激活）、`disabled`（已禁用）、`expired`（已过期）、`revoked`（已撤销）
- **activations**：激活记录（license_id、设备指纹、激活方式、最后验证时间）
- **offline_requests**：离线激活请求记录
- **products**：产品列表（产品标识、名称、描述）

## 授权码状态流转

```
active（激活） ←→ disabled（已禁用）
      ↓                ↓
   revoked（已撤销，不可逆）
      ↓
   expired（已过期）
```

- **active → disabled**：管理员手动禁用（临时停用，可恢复）
- **disabled → active**：管理员手动启用（恢复使用）
- **active/disabled → revoked**：管理员撤销（永久失效，不可逆）
- **active → expired**：超过有效期（自动失效）

## 申请授权码流程

适用于用户自助申请授权码的场景：

```
[用户访问申请页面]
  1. GET /api/v1/products（获取可申请的产品列表）
  2. 用户填写申请信息（姓名、邮箱、用途等）
  3. 用户选择产品（支持多选）
     └─> POST /api/v1/apply（提交申请）
           └─> 申请记录状态：pending（待审核）

[管理员审核]
  4. 管理员查看申请列表
  5. 管理员审批通过
     └─> 自动生成授权码（包含用户选择的所有产品）
     └─> 申请记录状态：approved（已通过）
  6. 管理员审批拒绝
     └─> 申请记录状态：rejected（已拒绝）

[用户获取授权码]
  7. 用户查看申请状态
     └─> GET /api/v1/apply/:applyId（查看申请详情和授权码）
```

## 环境变量

| 变量         | 默认值                     | 说明                   |
|------------|-------------------------|----------------------|
| DB_TYPE    | sqlite                  | 数据库类型：sqlite / mysql |
| DB_DSN     | license.db              | 数据库连接串               |
| HTTP_ADDR  | :8080                   | HTTP 监听地址            |
| JWT_SECRET | change-me-in-production | JWT 签名密钥             |

## 生产部署建议

1. 修改 `JWT_SECRET` 为强随机密钥
2. 使用 MySQL 替代 SQLite
3. 为离线激活配置 RSA 密钥文件（私钥用于签名，公钥内嵌在响应文件中分发给客户端）
4. 在生产环境将 `gin.ReleaseMode` 保留，开发时可改为 `gin.DebugMode`
5. 建议为管理员接口增加角色权限控制
6. **数据迁移**：从旧版本升级时，系统会自动将 `product_key` 列数据迁移到 `product_keys` 列（JSON 数组格式），无需手动操作
7. **禁用功能使用建议**：临时停用授权码请使用【禁用】功能，需要永久作废时使用【撤销】功能
