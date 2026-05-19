# Lic

基于 Golang + Gin + GORM 实现的软件授权验证服务端，支持在线和离线两种激活方式。

## 功能特性

- **帐密登录**：用户名/密码登录，返回 JWT Token
- **在线激活**：客户端启动时通过 Token + 设备指纹在线激活
- **离线激活**：支持离线环境下的授权激活（请求文件 + 响应文件机制）
- **授权码管理**：创建、查询、撤销、延期授权码
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

## 数据库表结构

- **users**：用户账号（用户名、密码哈希、邮箱）
- **licenses**：授权码（license_key、有效期、最大激活数、已激活数）
- **activations**：激活记录（license_id、设备指纹、激活方式、最后验证时间）
- **offline_requests**：离线激活请求记录

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
