# License SDK - Python 3

用于验证离线授权文件的 Python 3 SDK。

## 功能特性

- ✅ 验证离线授权文件（本地文件）
- ✅ 验证内存中的授权数据
- ✅ 简单的授权有效性检查接口
- ✅ 从服务器下载授权文件
- ✅ RSA 签名验证
- ✅ 纯 Python 实现，依赖少（仅需 `cryptography` 用于 RSA）

## 安装

### 环境要求

- Python 3.7+

### 安装依赖

```bash
cd sdk/python
pip install -r requirements.txt
```

或单独安装：

```bash
pip install cryptography>=3.4.8
pip install requests>=2.28.0  # 可选，仅下载功能需要
```

## 快速开始

### 1. 验证本地授权文件

```python
from verifier import verify_offline_auth_file, is_license_valid

# 服务器的 RSA 公钥（PEM 格式）
server_public_key = """-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwQ...
-----END PUBLIC KEY-----"""

# 方法1：详细验证
valid, reason, auth_file = verify_offline_auth_file(
    "license_auth.json",
    server_public_key
)

if valid:
    print(f"公司: {auth_file.company}")
    print(f"授权码: {auth_file.license_key}")
    print(f"有效期至: {auth_file.valid_to}")
else:
    print(f"无效: {reason}")

# 方法2：简单检查
valid, reason = is_license_valid("license_auth.json", server_public_key)
if valid:
    print("授权有效")
else:
    print(f"授权无效: {reason}")
```

### 2. 从服务器下载授权文件

```python
from verifier import download_offline_auth_file

server_url = "http://localhost:8080"
token = "your-jwt-token-here"
license_key = "YOUR-LICENSE-KEY"
save_path = "license_auth.json"

download_offline_auth_file(server_url, token, license_key, save_path)
print("授权文件下载成功")
```

### 3. 验证内存中的授权数据

```python
from verifier import verify_offline_auth_file_from_data, OfflineAuthFile
from datetime import datetime, timedelta

# 创建授权文件数据（实际应从文件或网络加载）
auth_file = OfflineAuthFile(
    version="1.0",
    license_key="DEMO-1234-5678",
    company="测试公司",
    product_keys=["product-a", "product-b"],
    valid_from=datetime.now() - timedelta(days=1),
    valid_to=datetime.now() + timedelta(days=365),
    activated_count=5,
    max_activations=10,
    issued_at=datetime.now(),
    signature="base64-encoded-signature",
    certificate=server_public_key
)

# 验证
valid, reason = verify_offline_auth_file_from_data(auth_file, server_public_key)

if valid:
    print("授权数据有效")
else:
    print(f"无效: {reason}")
```

## API 参考

### `verify_offline_auth_file(file_path, server_public_key_pem)`

验证磁盘上的离线授权文件。

**参数:**
- `file_path` (str): 授权文件路径（JSON 格式）
- `server_public_key_pem` (str): 服务器的 RSA 公钥（PEM 格式）

**返回:**
- `tuple`: `(valid: bool, reason: str, auth_file: OfflineAuthFile | None)`

**示例:**
```python
valid, reason, auth_file = verify_offline_auth_file("license.json", public_key)
```

---

### `verify_offline_auth_file_from_data(auth_file, server_public_key_pem)`

验证内存中的离线授权文件数据。

**参数:**
- `auth_file` (OfflineAuthFile): 授权文件对象
- `server_public_key_pem` (str): 服务器的 RSA 公钥（PEM 格式）

**返回:**
- `tuple`: `(valid: bool, reason: str)`

**示例:**
```python
valid, reason = verify_offline_auth_file_from_data(auth_file, public_key)
```

---

### `is_license_valid(file_path, server_public_key_pem)`

简单检查授权是否有效的接口。

**参数:**
- `file_path` (str): 授权文件路径
- `server_public_key_pem` (str): 服务器的 RSA 公钥（PEM 格式）

**返回:**
- `tuple`: `(valid: bool, reason: str)`

**示例:**
```python
valid, reason = is_license_valid("license.json", public_key)
if valid:
    print("授权有效")
```

---

### `download_offline_auth_file(server_url, token, license_key, save_path)`

从服务器下载离线授权文件。

**参数:**
- `server_url` (str): 服务器 URL（例如 `http://localhost:8080`）
- `token` (str): 认证令牌（JWT）
- `license_key` (str): 授权码
- `save_path` (str): 保存授权文件的路径

**返回:**
- `None`

**异常:**
- `Exception`: 下载失败时抛出异常

**示例:**
```python
download_offline_auth_file(
    "http://localhost:8080",
    "your-jwt-token",
    "YOUR-LICENSE-KEY",
    "license_auth.json"
)
```

---

### `OfflineAuthFile` 类

离线授权文件的数据结构。

**属性:**
- `version` (str): 版本号
- `license_key` (str): 授权码
- `company` (str): 公司或个人名称
- `product_keys` (List[str]): 产品密钥列表
- `valid_from` (datetime): 授权开始时间
- `valid_to` (datetime): 授权结束时间
- `activated_count` (int): 已激活设备数量
- `max_activations` (int): 最大激活数量
- `issued_at` (datetime): 签发时间
- `signature` (str): RSA 签名（base64 编码）
- `certificate` (str): 服务器公钥（PEM 格式）

**方法:**
- `from_dict(data: dict) -> OfflineAuthFile`: 从字典创建实例
- `to_dict() -> dict`: 转换为字典
- `get_sign_data() -> dict`: 获取用于签名的字段（排除 `signature` 和 `certificate`）

## 集成指南

### 集成到应用启动流程

```python
import os
from verifier import is_license_valid

LICENSE_FILE = "license_auth.json"
SERVER_PUBLIC_KEY = """-----BEGIN PUBLIC KEY-----
...
-----END PUBLIC KEY-----"""

def check_license():
    """应用启动时检查授权"""
    if not os.path.exists(LICENSE_FILE):
        print("❌ 未找到授权文件")
        return False
    
    valid, reason = is_license_valid(LICENSE_FILE, SERVER_PUBLIC_KEY)
    
    if not valid:
        print(f"❌ 授权无效: {reason}")
        return False
    
    print("✅ 授权验证通过，应用正常启动...")
    return True

# 应用入口
if __name__ == '__main__':
    if not check_license():
        exit(1)
    
    # 应用主逻辑
    print("应用运行中...")
```

### 定期授权检查（可选）

```python
import threading
import time

def periodic_license_check(interval_hours=24):
    """定期检查授权有效性"""
    def check():
        while True:
            valid, reason = is_license_valid(LICENSE_FILE, SERVER_PUBLIC_KEY)
            if not valid:
                print(f"⚠️ 授权检查失败: {reason}")
                # 采取相应措施（例如禁用功能、通知用户）
            time.sleep(interval_hours * 3600)
    
    thread = threading.Thread(target=check, daemon=True)
    thread.start()
```

## 离线授权工作流程

### 步骤1：下载授权文件（在线）

```python
# 从服务器下载（需要联网）
download_offline_auth_file(
    server_url="http://your-server:8080",
    token="user-jwt-token",
    license_key="YOUR-LICENSE-KEY",
    save_path="./license_auth.json"
)
```

### 步骤2：验证授权（在线或离线）

```python
# 本地验证（下载后无需联网）
valid, reason = is_license_valid("./license_auth.json", SERVER_PUBLIC_KEY)

if valid:
    print("授权有效，应用可以运行")
else:
    print(f"授权无效: {reason}")
    # 禁用应用或限制功能
```

### 步骤3：应用启动检查

```python
# 每次应用启动时检查授权
if not os.path.exists("license_auth.json"):
    print("请连接网络以下载授权文件")
    exit(1)

valid, reason = is_license_valid("license_auth.json", SERVER_PUBLIC_KEY)
if not valid:
    print(f"授权无效: {reason}")
    exit(1)

# 启动应用
main()
```

## 注意事项

1. **时钟篡改风险**：本 SDK 仅检查 `valid_from` 和 `valid_to` 时间戳。用户可以修改系统时钟来绕过过期检查。如果需要更强的保护，请考虑：
   - 使用安全时间源（例如带认证的 NTP）
   - 实现自定义防篡改措施

2. **公钥安全**：在应用中安全存储服务器的公钥。建议：
   - 嵌入到编译后的二进制文件中
   - 存储在加密配置中
   - 使用代码混淆技术

3. **签名验证**：SDK 使用 RSA-SHA256 和 PKCS1v15 填充。确保服务器使用相同的算法。

4. **文件权限**：设置适当的文件权限以防止授权文件被篡改：
   ```bash
   chmod 600 license_auth.json
   ```

## 示例代码

查看 `example.py` 获取完整的使用示例：

```bash
# 运行所有示例
python example.py all

# 运行特定示例
python example.py verify
python example.py simple
python example.py download
python example.py data
python example.py integrate
```

## 项目结构

```
sdk/python/
├── __init__.py          # 包初始化文件
├── verifier.py          # 核心验证函数
├── example.py           # 使用示例
├── test_sdk.py          # 功能测试脚本
├── requirements.txt     # 依赖文件
├── setup.py             # Python 包配置
└── README.md            # 本文件
```

## 许可证

MIT 许可证

## 支持

如有问题和疑问，请联系开发团队。
