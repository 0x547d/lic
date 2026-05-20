#!/usr/bin/env python3
"""
License SDK 使用示例
"""

import sys
import os

# 添加SDK路径到sys.path（如果在SDK目录外运行）
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from verifier import (
    verify_offline_auth_file,
    verify_offline_auth_file_from_data,
    is_license_valid,
    download_offline_auth_file,
    OfflineAuthFile
)


# 示例1: 验证本地授权文件
def example_verify_local_file():
    print("=" * 60)
    print("示例1: 验证本地授权文件")
    print("=" * 60)
    
    # 服务器公钥（实际使用时从服务器获取或配置）
    server_public_key = """-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwQ...
-----END PUBLIC KEY-----"""
    
    file_path = "license_auth.json"
    
    if not os.path.exists(file_path):
        print(f"授权文件不存在: {file_path}")
        print("请先运行示例3下载授权文件")
        return
    
    valid, reason, auth_file = verify_offline_auth_file(file_path, server_public_key)
    
    if valid:
        print("✅ 授权验证通过")
        print(f"公司: {auth_file.company}")
        print(f"授权码: {auth_file.license_key}")
        print(f"有效期: {auth_file.valid_from} 至 {auth_file.valid_to}")
        print(f"已激活: {auth_file.activated_count}/{auth_file.max_activations}")
        print(f"产品密钥: {', '.join(auth_file.product_keys)}")
    else:
        print(f"❌ 授权验证失败: {reason}")


# 示例2: 简单检查授权是否有效
def example_simple_check():
    print("\n" + "=" * 60)
    print("示例2: 简单检查授权是否有效")
    print("=" * 60)
    
    server_public_key = """-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwQ...
-----END PUBLIC KEY-----"""
    
    file_path = "license_auth.json"
    
    if not os.path.exists(file_path):
        print(f"授权文件不存在: {file_path}")
        return
    
    valid, reason = is_license_valid(file_path, server_public_key)
    
    if valid:
        print("✅ 授权有效")
    else:
        print(f"❌ 授权无效: {reason}")


# 示例3: 从服务器下载授权文件
def example_download_auth_file():
    print("\n" + "=" * 60)
    print("示例3: 从服务器下载授权文件")
    print("=" * 60)
    
    server_url = "http://localhost:8080"
    token = "your-jwt-token-here"
    license_key = "YOUR-LICENSE-KEY"
    save_path = "license_auth.json"
    
    print(f"正在从 {server_url} 下载授权文件...")
    print(f"授权码: {license_key}")
    print(f"保存路径: {save_path}")
    
    try:
        download_offline_auth_file(server_url, token, license_key, save_path)
        print("✅ 授权文件下载成功")
    except Exception as e:
        print(f"❌ 下载失败: {e}")


# 示例4: 验证授权文件数据（内存中）
def example_verify_from_data():
    print("\n" + "=" * 60)
    print("示例4: 验证授权文件数据（内存中）")
    print("=" * 60)
    
    import json
    from datetime import datetime, timedelta
    
    # 模拟授权文件数据（实际应从文件或网络加载）
    auth_file_data = {
        "version": "1.0",
        "license_key": "DEMO-1234-5678",
        "company": "测试公司",
        "product_keys": ["product-a", "product-b"],
        "valid_from": (datetime.now() - timedelta(days=1)).isoformat(),
        "valid_to": (datetime.now() + timedelta(days=365)).isoformat(),
        "activated_count": 5,
        "max_activations": 10,
        "issued_at": datetime.now().isoformat(),
        "signature": "base64-encoded-signature-here",
        "certificate": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----"
    }
    
    # 创建OfflineAuthFile对象
    auth_file = OfflineAuthFile.from_dict(auth_file_data)
    
    server_public_key = auth_file.certificate
    
    # 验证
    valid, reason = verify_offline_auth_file_from_data(auth_file, server_public_key)
    
    if valid:
        print("✅ 授权数据验证通过")
        print(f"公司: {auth_file.company}")
    else:
        print(f"❌ 授权数据验证失败: {reason}")


# 示例5: 集成到应用启动流程
def example_integrate_to_app():
    print("\n" + "=" * 60)
    print("示例5: 集成到应用启动流程")
    print("=" * 60)
    
    server_public_key = """-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwQ...
-----END PUBLIC KEY-----"""
    
    LICENSE_FILE = "license_auth.json"
    
    def check_license_on_startup():
        """应用启动时检查授权"""
        if not os.path.exists(LICENSE_FILE):
            print("❌ 未找到授权文件")
            print("请先运行: python example.py download")
            return False
        
        valid, reason = is_license_valid(LICENSE_FILE, server_public_key)
        
        if not valid:
            print(f"❌ 授权无效: {reason}")
            return False
        
        print("✅ 授权验证通过，应用正常启动")
        return True
    
    # 模拟应用启动
    if check_license_on_startup():
        print("\n应用主逻辑开始执行...")
        # 这里放应用的主要逻辑
    else:
        print("\n应用启动失败，请检查授权")


def main():
    """主函数"""
    import argparse
    
    parser = argparse.ArgumentParser(description='License SDK 使用示例')
    parser.add_argument('action', choices=['verify', 'simple', 'download', 'data', 'integrate', 'all'],
                       help='要运行的示例')
    
    args = parser.parse_args()
    
    if args.action == 'verify':
        example_verify_local_file()
    elif args.action == 'simple':
        example_simple_check()
    elif args.action == 'download':
        example_download_auth_file()
    elif args.action == 'data':
        example_verify_from_data()
    elif args.action == 'integrate':
        example_integrate_to_app()
    elif args.action == 'all':
        example_verify_local_file()
        example_simple_check()
        example_verify_from_data()
        example_integrate_to_app()


if __name__ == '__main__':
    # 如果没有参数，显示帮助
    import sys
    if len(sys.argv) == 1:
        print("License SDK 使用示例")
        print("\n使用方法:")
        print("  python example.py verify    - 验证本地授权文件")
        print("  python example.py simple    - 简单检查授权")
        print("  python example.py download  - 下载授权文件")
        print("  python example.py data      - 验证内存中的数据")
        print("  python example.py integrate  - 集成示例")
        print("  python example.py all       - 运行所有示例")
    else:
        main()
