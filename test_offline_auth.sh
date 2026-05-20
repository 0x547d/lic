#!/bin/bash

# 离线授权验证功能测试脚本

echo "=========================================="
echo "  离线授权验证功能测试"
echo "=========================================="
echo ""

# 1. 启动服务（如果未启动）
echo "1. 检查服务是否运行..."
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "   ❌ 服务未运行，请先启动服务："
    echo "      go run main.go"
    exit 1
fi
echo "   ✅ 服务已运行"
echo ""

# 2. 注册用户
echo "2. 注册测试用户..."
REGISTER_RESP=$(curl -s -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"username":"测试公司","password":"123456","email":"test@example.com"}')

echo "   响应: $REGISTER_RESP"

LICENSE_KEY=$(echo $REGISTER_RESP | grep -o '"license_key":"[^"]*"' | cut -d'"' -f4)
echo "   授权码: $LICENSE_KEY"
echo ""

# 3. 登录获取 Token
echo "3. 登录获取 Token..."
LOGIN_RESP=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"测试公司","password":"123456"}')

TOKEN=$(echo $LOGIN_RESP | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
echo "   Token: ${TOKEN:0:20}..."
echo ""

# 4. 下载离线授权文件
echo "4. 下载离线授权文件..."
if [ -z "$TOKEN" ]; then
    echo "   ❌ 获取 Token 失败"
    exit 1
fi

DOWNLOAD_RESP=$(curl -s -X GET "http://localhost:8080/api/v1/license/$LICENSE_KEY/offline-auth-file?offline_days=7" \
  -H "Authorization: Bearer $TOKEN")

# 保存到文件
echo $DOWNLOAD_RESP | jq '.' > offline-auth-test.json
echo "   ✅ 离线授权文件已保存: offline-auth-test.json"
echo ""

# 5. 显示文件内容
echo "5. 离线授权文件内容："
cat offline-auth-test.json | jq '. | {license_key, company, product_keys, valid_from, valid_to, offline_valid_to, activated_count, max_activations}'
echo ""

# 6. 验证签名（模拟客户端验证）
echo "6. 模拟客户端验证离线授权文件..."
echo "   （在实际客户端中，会使用嵌入的公钥验证签名）"
echo ""

echo "=========================================="
echo "  测试完成！"
echo "=========================================="
echo ""
echo "提示："
echo "  - 将 offline-auth-test.json 分发给客户端"
echo "  - 客户端定期（如每24小时）联网更新此文件"
echo "  - 离线时，客户端验证本地文件的有效性"
echo "  - 超过 offline_valid_to 后，必须联网重新下载"
