# License SDK for Go

Golang 客户端 SDK，用于验证离线授权文件。

## 📦 功能特性

- ✅ 验证离线授权文件（RSA 签名验证）
- ✅ 检查授权有效期（`valid_from` / `valid_to`）
- ✅ 从文件或内存数据验证授权
- ✅ 下载离线授权文件（需要联网）
- ✅ 无需离线宽限期，授权文件在有效期内始终有效

## 📦 安装

```bash
go get github.com/0x547d/lic/sdk/go
```

## 🚀 快速开始

### 示例 1：验证离线授权文件

```go
package main

import (
    "fmt"
    "os"

    lic_sdk "github.com/0x547d/lic/sdk/go"
)

func main() {
    // 验证离线授权文件
    result, err := lic_sdk.VerifyOfflineAuthFile("./offline-auth-A1B2C3D4.json")
    if err != nil {
        fmt.Printf("验证失败: %v\n", err)
        os.Exit(1)
    }

    if !result.Valid {
        fmt.Printf("授权无效: %s\n", result.Reason)
        os.Exit(1)
    }

    // 输出授权信息
    fmt.Println("✅ 授权验证通过")
    fmt.Printf("公司/个人名称: %s\n", result.License.Company)
    fmt.Printf("授权码:         %s\n", result.License.LicenseKey)
    fmt.Printf("产品列表:         %v\n", result.License.ProductKeys)
    fmt.Printf("有效期开始:       %s\n", result.License.ValidFrom.Format("2006-01-02"))
    fmt.Printf("有效期结束:       %s\n", result.License.ValidTo.Format("2006-01-02"))
    fmt.Printf("已激活设备:       %d / %d\n", result.License.ActivatedCount, result.License.MaxActivations)
}
```

### 示例 2：快速检查授权是否有效

```go
package main

import (
    "fmt"
    "os"

    lic_sdk "github.com/0x547d/lic/sdk/go"
)

func main() {
    valid, reason := lic_sdk.IsLicenseValid("./offline-auth-A1B2C3D4.json")
    if !valid {
        fmt.Printf("授权无效: %s\n", reason)
        os.Exit(1)
    }

    fmt.Println("✅ 授权有效")
}
```

### 示例 3：从内存数据验证授权

```go
package main

import (
    "fmt"
    "os"
    "os/exec"

    lic_sdk "github.com/0x547d/lic/sdk/go"
)

func main() {
    // 从数据库或配置中读取授权文件内容
    data := []byte(`{
        "version": "1.0",
        "license_key": "A1B2-C3D4-E5F6-7890",
        ...
    }`)

    // 验证授权
    result, err := lic_sdk.VerifyOfflineAuthFileFromData(data)
    if err != nil {
        fmt.Printf("验证失败: %v\n", err)
        os.Exit(1)
    }

    if !result.Valid {
        fmt.Printf("授权无效: %s\n", result.Reason)
        os.Exit(1)
    }

    fmt.Println("✅ 授权验证通过")
}
```

### 示例 4：从服务器下载离线授权文件

```go
package main

import (
    "fmt"
    "os"

    lic_sdk "github.com/0x547d/lic/sdk/go"
)

func main() {
    serverURL := "http://localhost:8080"
    token := "eyJhbGciOi..."
    licenseKey := "A1B2-C3D4-E5F6-7890"
    savePath := "./offline-auth-A1B2C3D4.json"

    // 下载离线授权文件
    err := lic_sdk.DownloadOfflineAuthFile(serverURL, token, licenseKey, savePath)
    if err != nil {
        fmt.Printf("下载失败: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("✅ 授权文件已保存: %s\n", savePath)
}
```

## 📋 API 文档

### `VerifyOfflineAuthFile(filePath string) (*VerifyResult, error)`

验证离线授权文件。

**参数**:
- `filePath`: 离线授权文件路径

**返回**:
- `*VerifyResult`: 验证结果
  - `Valid`: 是否有效（`bool`）
  - `Reason`: 无效原因（`string`）
  - `License`: 授权信息（`*OfflineAuthFile`）
- `error`: 错误信息

---

### `VerifyOfflineAuthFileFromData(data []byte) (*VerifyResult, error)`

从内存数据验证离线授权文件。

**参数**:
- `data`: JSON 格式的授权文件内容

**返回**:
- `*VerifyResult`: 验证结果
- `error`: 错误信息

---

### `IsLicenseValid(filePath string) (bool, string)`

快速检查授权是否有效（简化 API）。

**参数**:
- `filePath`: 离线授权文件路径

**返回**:
- `bool`: 是否有效
- `string`: 无效原因（如果有效则为空字符串）

---

### `DownloadOfflineAuthFile(serverURL, token, licenseKey, savePath string) error`

从服务器下载离线授权文件（需要联网）。

**参数**:
- `serverURL`: 服务器地址（如 `http://localhost:8080`）
- `token`: JWT Token
- `licenseKey`: 授权码
- `savePath`: 保存路径

**返回**:
- `error`: 错误信息

---

## 📋 数据结构

### `OfflineAuthFile`

```go
type OfflineAuthFile struct {
    Version        string    `json:"version"`        // 协议版本
    LicenseKey     string    `json:"license_key"`    // 授权码
    Company        string    `json:"company"`        // 公司/个人名称
    ProductKeys    []string  `json:"product_keys"`   // 授权产品列表
    ValidFrom      time.Time `json:"valid_from"`     // 授权有效期开始
    ValidTo        time.Time `json:"valid_to"`       // 授权有效期结束
    ActivatedCount int       `json:"activated_count"` // 已激活设备数
    MaxActivations int       `json:"max_activations"` // 最大激活数
    IssuedAt       time.Time `json:"issued_at"`      // 文件签发时间
    Signature       string    `json:"signature"`      // 服务端 RSA 签名
    Certificate     string    `json:"certificate"`    // 服务端公钥证书（PEM格式）
}
```

### `VerifyResult`

```go
type VerifyResult struct {
    Valid   bool             `json:"valid"`    // 是否有效
    Reason  string           `json:"reason"`   // 无效原因
    License *OfflineAuthFile `json:"license"` // 授权信息
}
```

## ⚠️ 安全建议

1. **公钥管理**：
   - 将服务端公钥（`Certificate` 字段）硬编码到客户端程序中
   - 不要从网络动态获取公钥

2. **时钟篡改防护**：
   - 本 SDK 使用**方案1**（只校验授权有效期，无离线宽限期）
   - **风险**：用户可以修改系统时钟绕过授权过期检查
   - **建议**：如果安全性要求高，请使用**方案3**（保留离线宽限期，定期联网）

3. **文件完整性**：
   - 不要将离线授权文件存储在可被用户轻易修改的位置
   - 建议对文件进行额外的哈希校验

## 📦 完整示例

参见 `example/main.go`：

```bash
# 1. 下载离线授权文件（需要 JWT Token）
go run example/main.go download <token> <license_key>

# 2. 验证离线授权文件（无需联网）
go run example/main.go verify <license_key>
```

## 🔗 相关链接

- **服务端项目**: [license-server](https://github.com/0x547d/lic)
- **API 文档**: 参见服务端 `README.md`

## 📄 许可证

MIT License
