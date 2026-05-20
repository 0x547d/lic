package lic_sdk

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// VerifyOfflineAuthFile 验证离线授权文件（主要 API）
// 参数：
//   - filePath: 离线授权文件路径
//
// 返回：
//   - *VerifyResult: 验证结果（包含是否有效、原因、授权信息）
//   - error: 错误信息
func VerifyOfflineAuthFile(filePath string) (*VerifyResult, error) {
	// 1. 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 2. 解析 JSON
	var authFile OfflineAuthFile
	if err := json.Unmarshal(data, &authFile); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 3. 验证授权文件
	valid, reason, err := verifyAuthFile(&authFile)

	result := &VerifyResult{
		Valid:   valid,
		Reason:  reason,
		License: &authFile,
	}

	return result, err
}

// VerifyOfflineAuthFileFromData 从内存数据验证离线授权文件
// 参数：
//   - data: JSON 格式的授权文件内容
//
// 返回：
//   - *VerifyResult: 验证结果
//   - error: 错误信息
func VerifyOfflineAuthFileFromData(data []byte) (*VerifyResult, error) {
	// 1. 解析 JSON
	var authFile OfflineAuthFile
	if err := json.Unmarshal(data, &authFile); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 2. 验证授权文件
	valid, reason, err := verifyAuthFile(&authFile)

	result := &VerifyResult{
		Valid:   valid,
		Reason:  reason,
		License: &authFile,
	}

	return result, err
}

// verifyAuthFile 验证离线授权文件（内部函数）
func verifyAuthFile(authFile *OfflineAuthFile) (bool, string, error) {
	// 1. 验证时间有效性（授权有效期）
	now := time.Now()
	if now.Before(authFile.ValidFrom) {
		return false, "license not yet valid", nil
	}
	if now.After(authFile.ValidTo) {
		return false, "license expired", nil
	}

	// 2. 验证签名
	valid, err := verifySignature(authFile)
	if err != nil {
		return false, fmt.Sprintf("signature verification error: %v", err), err
	}
	if !valid {
		return false, "invalid signature", nil
	}

	return true, "", nil
}

// verifySignature 验证离线授权文件的 RSA 签名
func verifySignature(authFile *OfflineAuthFile) (bool, error) {
	// 1. 获取签名数据
	signData := getAuthFileSignData(authFile)

	// 2. 解析公钥
	pubKey, err := parsePublicKey(authFile.Certificate)
	if err != nil {
		return false, err
	}

	// 3. 计算哈希
	hash := sha256.Sum256([]byte(signData))

	// 4. 解码签名
	sig, err := hex.DecodeString(authFile.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature encoding: %w", err)
	}

	// 5. 验证签名
	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sig)
	return err == nil, err
}

// getAuthFileSignData 获取离线授权文件签名数据
func getAuthFileSignData(authFile *OfflineAuthFile) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d|%d",
		authFile.Version,
		authFile.LicenseKey,
		authFile.Company,
		authFile.ValidFrom.Format(time.RFC3339),
		authFile.ValidTo.Format(time.RFC3339),
		authFile.ActivatedCount,
		authFile.MaxActivations,
	)
}

// parsePublicKey 解析 PEM 格式公钥
func parsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// DownloadOfflineAuthFile 从服务器下载离线授权文件（需要联网）
// 参数：
//   - serverURL: 服务器地址（如 http://localhost:8080）
//   - token: JWT Token
//   - licenseKey: 授权码
//   - savePath: 保存路径
//
// 返回：
//   - error: 错误信息
func DownloadOfflineAuthFile(serverURL, token, licenseKey, savePath string) error {
	// 构建 URL
	url := fmt.Sprintf("%s/api/v1/license/%s/offline-auth-file", serverURL, licenseKey)

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 Token
	req.Header.Set("Authorization", "Bearer "+token)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// 保存到文件
	if err := os.WriteFile(savePath, body, 0644); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// IsLicenseValid 快速检查授权是否有效（简化 API）
// 参数：
//   - filePath: 离线授权文件路径
//
// 返回：
//   - bool: 是否有效
//   - string: 无效原因（如果有效则为空字符串）
func IsLicenseValid(filePath string) (bool, string) {
	result, err := VerifyOfflineAuthFile(filePath)
	if err != nil {
		return false, err.Error()
	}
	return result.Valid, result.Reason
}
