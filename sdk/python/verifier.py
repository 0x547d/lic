"""
License SDK - 离线授权文件验证模块
"""

import json
import os
import base64
from dataclasses import dataclass, field
from datetime import datetime
from typing import List, Optional, Tuple

try:
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import padding
    from cryptography.exceptions import InvalidSignature
    HAS_CRYPTOGRAPHY = True
except ImportError:
    HAS_CRYPTOGRAPHY = False


@dataclass
class OfflineAuthFile:
    """离线授权文件数据结构"""
    version: str
    license_key: str
    company: str
    product_keys: List[str]
    valid_from: datetime
    valid_to: datetime
    activated_count: int
    max_activations: int
    issued_at: datetime
    signature: str
    certificate: str

    @classmethod
    def from_dict(cls, data: dict) -> 'OfflineAuthFile':
        """从字典创建实例"""
        return cls(
            version=data.get('version', ''),
            license_key=data.get('license_key', ''),
            company=data.get('company', ''),
            product_keys=data.get('product_keys', []),
            valid_from=datetime.fromisoformat(data.get('valid_from', '').replace('Z', '+00:00')),
            valid_to=datetime.fromisoformat(data.get('valid_to', '').replace('Z', '+00:00')),
            activated_count=data.get('activated_count', 0),
            max_activations=data.get('max_activations', 0),
            issued_at=datetime.fromisoformat(data.get('issued_at', '').replace('Z', '+00:00')),
            signature=data.get('signature', ''),
            certificate=data.get('certificate', '')
        )

    def to_dict(self) -> dict:
        """转换为字典"""
        return {
            'version': self.version,
            'license_key': self.license_key,
            'company': self.company,
            'product_keys': self.product_keys,
            'valid_from': self.valid_from.isoformat() if self.valid_from else None,
            'valid_to': self.valid_to.isoformat() if self.valid_to else None,
            'activated_count': self.activated_count,
            'max_activations': self.max_activations,
            'issued_at': self.issued_at.isoformat() if self.issued_at else None,
            'signature': self.signature,
            'certificate': self.certificate
        }

    def get_sign_data(self) -> dict:
        """获取用于签名的字段（排除signature和certificate）"""
        data = self.to_dict()
        del data['signature']
        del data['certificate']
        return data


@dataclass
class VerifyResult:
    """验证结果"""
    valid: bool
    reason: str
    license: Optional[OfflineAuthFile] = None


def _load_public_key(pem_data: str):
    """加载RSA公钥"""
    if not HAS_CRYPTOGRAPHY:
        raise ImportError("cryptography library is required for RSA signature verification")
    
    pem_bytes = pem_data.encode('utf-8')
    return serialization.load_pem_public_key(pem_bytes)


def _verify_signature(auth_file: OfflineAuthFile, public_key) -> bool:
    """验证RSA签名"""
    if not HAS_CRYPTOGRAPHY:
        raise ImportError("cryptography library is required for RSA signature verification")
    
    # 获取签名数据
    sign_data = auth_file.get_sign_data()
    data_str = json.dumps(sign_data, sort_keys=True, separators=(',', ':'))
    data_bytes = data_str.encode('utf-8')
    
    # Base64解码签名
    signature_bytes = base64.b64decode(auth_file.signature)
    
    # 验证签名
    try:
        public_key.verify(
            signature_bytes,
            data_bytes,
            padding.PKCS1v15(),
            hashes.SHA256()
        )
        return True
    except InvalidSignature:
        return False


def verify_offline_auth_file_from_data(auth_file: OfflineAuthFile, server_public_key_pem: str) -> Tuple[bool, str]:
    """
    验证离线授权文件数据
    
    Args:
        auth_file: 离线授权文件对象
        server_public_key_pem: 服务器RSA公钥PEM格式
    
    Returns:
        (valid, reason): 是否有效及原因
    """
    # 检查有效期
    now = datetime.now()
    
    if now < auth_file.valid_from:
        return False, "license not yet valid"
    
    if now > auth_file.valid_to:
        return False, "license expired"
    
    # 验证RSA签名
    try:
        public_key = _load_public_key(server_public_key_pem)
    except Exception as e:
        return False, f"invalid server public key: {e}"
    
    try:
        signature_valid = _verify_signature(auth_file, public_key)
    except Exception as e:
        return False, f"signature verification failed: {e}"
    
    if not signature_valid:
        return False, "invalid signature"
    
    return True, ""


def verify_offline_auth_file(file_path: str, server_public_key_pem: str) -> Tuple[bool, str, Optional[OfflineAuthFile]]:
    """
    验证离线授权文件
    
    Args:
        file_path: 授权文件路径
        server_public_key_pem: 服务器RSA公钥PEM格式
    
    Returns:
        (valid, reason, auth_file): 是否有效、原因及授权文件对象
    """
    # 读取文件
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            data = json.load(f)
    except Exception as e:
        return False, f"failed to read file: {e}", None
    
    # 解析授权文件
    try:
        auth_file = OfflineAuthFile.from_dict(data)
    except Exception as e:
        return False, f"failed to parse auth file: {e}", None
    
    # 验证授权文件
    valid, reason = verify_offline_auth_file_from_data(auth_file, server_public_key_pem)
    
    return valid, reason, auth_file


def is_license_valid(file_path: str, server_public_key_pem: str) -> Tuple[bool, str]:
    """
    简单检查授权是否有效
    
    Args:
        file_path: 授权文件路径
        server_public_key_pem: 服务器RSA公钥PEM格式
    
    Returns:
        (valid, reason): 是否有效及原因
    """
    valid, reason, _ = verify_offline_auth_file(file_path, server_public_key_pem)
    return valid, reason


def download_offline_auth_file(server_url: str, token: str, license_key: str, save_path: str) -> None:
    """
    从服务器下载离线授权文件
    
    Args:
        server_url: 服务器URL (e.g., http://localhost:8080)
        token: 认证token
        license_key: 授权码
        save_path: 保存路径
    
    Raises:
        Exception: 下载失败时抛出异常
    """
    try:
        import requests
    except ImportError:
        raise ImportError("requests library is required for downloading auth file")
    
    url = f"{server_url}/api/v1/license/{license_key}/offline-auth-file"
    
    headers = {
        'Authorization': f'Bearer {token}'
    }
    
    try:
        response = requests.get(url, headers=headers, timeout=30)
    except Exception as e:
        raise Exception(f"failed to send request: {e}")
    
    if response.status_code != 200:
        raise Exception(f"server returned {response.status_code}: {response.text}")
    
    # 保存文件
    try:
        with open(save_path, 'w', encoding='utf-8') as f:
            f.write(response.text)
    except Exception as e:
        raise Exception(f"failed to save file: {e}")


# 导出
__all__ = [
    'OfflineAuthFile',
    'VerifyResult',
    'verify_offline_auth_file',
    'verify_offline_auth_file_from_data',
    'is_license_valid',
    'download_offline_auth_file'
]
