#!/usr/bin/env python3
"""
测试 License SDK 功能
"""

import os
import json
import base64
from datetime import datetime, timedelta
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.backends import default_backend

# 添加当前目录到path
import sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from verifier import (
    OfflineAuthFile,
    verify_offline_auth_file_from_data,
    verify_offline_auth_file,
    is_license_valid
)


def generate_test_keys():
    """生成测试用的RSA密钥对"""
    print("生成测试RSA密钥对...")
    
    # 生成私钥
    private_key = rsa.generate_private_key(
        public_exponent=65537,
        key_size=2048,
        backend=default_backend()
    )
    
    # 序列化私钥
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption()
    ).decode('utf-8')
    
    # 序列化公钥
    public_key = private_key.public_key()
    public_pem = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo
    ).decode('utf-8')
    
    print("✅ 密钥对生成成功")
    return private_key, private_pem, public_pem


def sign_auth_file(auth_file_dict, private_key):
    """对授权文件数据签名"""
    # 准备签名数据（排除signature和certificate）
    sign_data = auth_file_dict.copy()
    del sign_data['signature']
    del sign_data['certificate']
    
    data_str = json.dumps(sign_data, sort_keys=True, separators=(',', ':'))
    data_bytes = data_str.encode('utf-8')
    
    # 签名
    signature = private_key.sign(
        data_bytes,
        padding.PKCS1v15(),
        hashes.SHA256()
    )
    
    # Base64编码
    return base64.b64encode(signature).decode('utf-8')


def create_test_auth_file(private_key, public_pem):
    """创建测试授权文件"""
    print("\n创建测试授权文件...")
    
    now = datetime.now()
    
    auth_file_dict = {
        "version": "1.0",
        "license_key": "TEST-1234-5678-9012",
        "company": "测试公司",
        "product_keys": ["product-a", "product-b"],
        "valid_from": (now - timedelta(days=1)).isoformat(),
        "valid_to": (now + timedelta(days=365)).isoformat(),
        "activated_count": 3,
        "max_activations": 10,
        "issued_at": now.isoformat(),
        "signature": "",
        "certificate": public_pem
    }
    
    # 签名
    signature = sign_auth_file(auth_file_dict, private_key)
    auth_file_dict['signature'] = signature
    
    # 保存到文件
    test_file = "test_license_auth.json"
    with open(test_file, 'w', encoding='utf-8') as f:
        json.dump(auth_file_dict, f, indent=2, ensure_ascii=False)
    
    print(f"✅ 测试授权文件已保存: {test_file}")
    return test_file, auth_file_dict


def test_verify_from_data(public_pem):
    """测试验证内存中的数据"""
    print("\n" + "="*60)
    print("测试1: 验证内存中的授权数据")
    print("="*60)
    
    # 创建测试数据
    now = datetime.now()
    auth_file = OfflineAuthFile(
        version="1.0",
        license_key="TEST-1234-5678",
        company="测试公司",
        product_keys=["prod-a", "prod-b"],
        valid_from=now - timedelta(days=1),
        valid_to=now + timedelta(days=365),
        activated_count=2,
        max_activations=5,
        issued_at=now,
        signature="dummy-signature",  # 会被拒绝
        certificate=public_pem
    )
    
    # 测试无效签名
    valid, reason = verify_offline_auth_file_from_data(auth_file, public_pem)
    print(f"测试无效签名:")
    print(f"  结果: {'✅ 通过' if not valid else '❌ 失败'} (期望: 无效)")
    print(f"  原因: {reason}")
    
    print("\n提示: 要使用有效签名测试，需要先生成签名")
    print("运行完整测试流程...")


def test_verify_from_file(test_file, public_pem):
    """测试验证文件"""
    print("\n" + "="*60)
    print("测试2: 验证授权文件")
    print("="*60)
    
    valid, reason, auth_file = verify_offline_auth_file(test_file, public_pem)
    
    print(f"验证结果:")
    print(f"  是否有效: {'✅ 是' if valid else '❌ 否'}")
    print(f"  原因: {reason}")
    
    if auth_file:
        print(f"\n授权信息:")
        print(f"  公司: {auth_file.company}")
        print(f"  授权码: {auth_file.license_key}")
        print(f"  有效期: {auth_file.valid_from} 至 {auth_file.valid_to}")
        print(f"  已激活: {auth_file.activated_count}/{auth_file.max_activations}")


def test_is_license_valid(test_file, public_pem):
    """测试简单检查接口"""
    print("\n" + "="*60)
    print("测试3: 简单检查授权是否有效")
    print("="*60)
    
    valid, reason = is_license_valid(test_file, public_pem)
    
    print(f"检查结果:")
    print(f"  是否有效: {'✅ 是' if valid else '❌ 否'}")
    print(f"  原因: {reason}")


def test_expired_license(public_pem):
    """测试过期授权"""
    print("\n" + "="*60)
    print("测试4: 验证过期授权")
    print("="*60)
    
    now = datetime.now()
    auth_file = OfflineAuthFile(
        version="1.0",
        license_key="EXPIRED-1234",
        company="过期测试",
        product_keys=["prod-a"],
        valid_from=now - timedelta(days=365),
        valid_to=now - timedelta(days=1),  # 已过期
        activated_count=1,
        max_activations=5,
        issued_at=now - timedelta(days=365),
        signature="dummy",
        certificate=public_pem
    )
    
    valid, reason = verify_offline_auth_file_from_data(auth_file, public_pem)
    
    print(f"测试过期授权:")
    print(f"  结果: {'✅ 通过' if not valid else '❌ 失败'} (期望: 无效)")
    print(f"  原因: {reason}")


def main():
    """主测试函数"""
    print("="*60)
    print("License SDK 功能测试")
    print("="*60)
    
    # 生成测试密钥
    private_key, private_pem, public_pem = generate_test_keys()
    
    # 创建测试授权文件
    test_file, auth_file_dict = create_test_auth_file(private_key, public_pem)
    
    # 运行测试
    test_verify_from_file(test_file, public_pem)
    test_is_license_valid(test_file, public_pem)
    test_verify_from_data(public_pem)
    test_expired_license(public_pem)
    
    print("\n" + "="*60)
    print("测试完成")
    print("="*60)
    print(f"\n测试文件: {test_file}")
    print("可以手动检查该文件内容")


if __name__ == '__main__':
    main()
