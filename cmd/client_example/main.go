package main

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

// OfflineAuthFile 离线授权文件结构（与服务器端 models.OfflineAuthFile 对应）
type OfflineAuthFile struct {
	Version        string    `json:"version"`
	LicenseKey     string    `json:"license_key"`
	Company        string    `json:"company"`
	ProductKeys    []string  `json:"product_keys"`
	ValidFrom      time.Time `json:"valid_from"`
	ValidTo        time.Time `json:"valid_to"`
	OfflineValidTo time.Time `json:"offline_valid_to"`
	ActivatedCount int       `json:"activated_count"`
	MaxActivations int       `json:"max_activations"`
	IssuedAt       time.Time `json:"issued_at"`
	Signature      string    `json:"signature"`
	Certificate    string    `json:"certificate"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  下载离线授权文件: go run client_example.go download <token> <license_key>")
		fmt.Println("  验证离线授权文件: go run client_example.go verify <license_key>")
		fmt.Println("")
		fmt.Println("示例:")
		fmt.Println("  go run client_example.go download eyJhbGci... A1B2-C3D4-E5F6-7890")
		fmt.Println("  go run client_example.go verify A1B2-C3D4-E5F6-7890")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "download":
		if len(os.Args) < 4 {
			fmt.Println("错误: 需要提供 token 和 license_key")
			os.Exit(1)
		}
		token := os.Args[2]
		licenseKey := os.Args[3]
		downloadOfflineAuthFile(token, licenseKey)

	case "verify":
		if len(os.Args) < 3 {
			fmt.Println("错误: 需要提供 license_key")
			os.Exit(1)
		}
		licenseKey := os.Args[2]
		verifyOfflineAuthFile(licenseKey)

	default:
		fmt.Printf("未知命令: %s\n", command)
		os.Exit(1)
	}
}

// downloadOfflineAuthFile 下载离线授权文件
func downloadOfflineAuthFile(token, licenseKey string) {
	url := fmt.Sprintf("http://localhost:8080/api/v1/license/%s/offline-auth-file", licenseKey)

	fmt.Printf("正在下载离线授权文件: %s\n", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		os.Exit(1)
	}

	// 设置 JWT Token
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("下载失败: HTTP %d - %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		os.Exit(1)
	}

	// 保存到本地文件
	filename := fmt.Sprintf("offline-auth-%s.json", licenseKey)
	err = os.WriteFile(filename, body, 0644)
	if err != nil {
		fmt.Printf("保存文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 离线授权文件已保存: %s\n", filename)
	fmt.Println("")
	fmt.Println("提示: 在离线环境中，使用 'verify' 命令验证此文件")
}

// verifyOfflineAuthFile 验证离线授权文件
func verifyOfflineAuthFile(licenseKey string) {
	filename := fmt.Sprintf("offline-auth-%s.json", licenseKey)

	fmt.Printf("正在验证离线授权文件: %s\n", filename)

	// 读取本地文件
	body, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("❌ 读取文件失败: %v\n", err)
		fmt.Println("提示: 请先使用 'download' 命令下载离线授权文件")
		os.Exit(1)
	}

	// 解析 JSON
	var authFile OfflineAuthFile
	err = json.Unmarshal(body, &authFile)
	if err != nil {
		fmt.Printf("❌ 解析文件失败: %v\n", err)
		os.Exit(1)
	}

	// 验证授权文件
	valid, reason, err := verifyAuthFile(&authFile)
	if err != nil {
		fmt.Printf("❌ 验证失败: %v\n", err)
		os.Exit(1)
	}

	if !valid {
		fmt.Printf("❌ 授权无效: %s\n", reason)
		os.Exit(1)
	}

	// 输出授权信息
	fmt.Println("✅ 授权验证通过")
	fmt.Println("")
	fmt.Printf("公司/个人名称: %s\n", authFile.Company)
	fmt.Printf("授权码:         %s\n", authFile.LicenseKey)
	fmt.Printf("产品列表:         %v\n", authFile.ProductKeys)
	fmt.Printf("有效期开始:       %s\n", authFile.ValidFrom.Format("2006-01-02"))
	fmt.Printf("有效期结束:       %s\n", authFile.ValidTo.Format("2006-01-02"))
	fmt.Printf("离线宽限期:       %s\n", authFile.OfflineValidTo.Format("2006-01-02"))
	fmt.Printf("已激活设备:       %d / %d\n", authFile.ActivatedCount, authFile.MaxActivations)
	fmt.Printf("剩余额度:         %d\n", authFile.MaxActivations-authFile.ActivatedCount)
	fmt.Println("")
	fmt.Println("提示: 离线宽限期过后，需要联网重新下载授权文件")
}

// verifyAuthFile 验证离线授权文件（客户端本地验证）
func verifyAuthFile(authFile *OfflineAuthFile) (bool, string, error) {
	// 1. 验证时间有效性（授权有效期）
	now := time.Now()
	if now.Before(authFile.ValidFrom) {
		return false, "license not yet valid", nil
	}
	if now.After(authFile.ValidTo) {
		return false, "license expired", nil
	}

	// 2. 验证离线宽限期
	if now.After(authFile.OfflineValidTo) {
		return false, "offline grace period expired, please connect to internet", nil
	}

	// 3. 验证签名（使用服务端公钥）
	valid, err := verifySignature(authFile, authFile.Certificate)
	if err != nil {
		return false, fmt.Sprintf("signature verification error: %v", err), err
	}
	if !valid {
		return false, "invalid signature", nil
	}

	return true, "", nil
}

// verifySignature 验证离线授权文件的 RSA 签名
func verifySignature(authFile *OfflineAuthFile, publicKeyPEM string) (bool, error) {
	// 获取签名数据
	signData := getAuthFileSignData(authFile)

	// 解析公钥
	pubKey, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return false, err
	}

	// 计算哈希
	hash := sha256.Sum256([]byte(signData))

	// 解码签名
	sig, err := hex.DecodeString(authFile.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature encoding: %w", err)
	}

	// 验证签名
	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sig)
	return err == nil, err
}

// getAuthFileSignData 获取离线授权文件签名数据
func getAuthFileSignData(authFile *OfflineAuthFile) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d|%s",
		authFile.Version,
		authFile.LicenseKey,
		authFile.Company,
		authFile.ValidFrom.Format(time.RFC3339),
		authFile.ValidTo.Format(time.RFC3339),
		authFile.ActivatedCount,
		authFile.MaxActivations,
		authFile.OfflineValidTo.Format(time.RFC3339),
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
