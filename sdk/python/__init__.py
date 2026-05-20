"""
License SDK for Python 3
用于验证离线授权文件
"""

from .verifier import (
    OfflineAuthFile,
    VerifyResult,
    verify_offline_auth_file,
    verify_offline_auth_file_from_data,
    is_license_valid,
    download_offline_auth_file
)

__version__ = "1.0.0"
__all__ = [
    'OfflineAuthFile',
    'VerifyResult',
    'verify_offline_auth_file',
    'verify_offline_auth_file_from_data',
    'is_license_valid',
    'download_offline_auth_file'
]
