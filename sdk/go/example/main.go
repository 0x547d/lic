package main

import (
	"fmt"
	"os"

	lic_sdk "github.com/0x547d/lic/sdk/go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  verify <auth_file_path>      - 验证离线授权文件")
		fmt.Println("  download <token> <license_key> - 从服务器下载授权文件")
		fmt.Println("")
		fmt.Println("示例:")
		fmt.Println("  go run main.go verify ./offline-auth-A1B2C3D4.json")
		fmt.Println("  go run main.go download <token> A1B2-C3D4-E5F6-7890")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "verify":
		if len(os.Args) < 3 {
			fmt.Println("错误: 需要提供授权文件路径")
			os.Exit(1)
		}
		filePath := os.Args[2]
		verifyLicense(filePath)

	case "download":
		if len(os.Args) < 4 {
			fmt.Println("错误: 需要提供 token 和 license_key")
			os.Exit(1)
		}
		token := os.Args[2]
		licenseKey := os.Args[3]
		downloadLicense(token, licenseKey)

	default:
		fmt.Printf("未知命令: %s\n", command)
		os.Exit(1)
	}
}

// verifyLicense 验证离线授权文件
func verifyLicense(filePath string) {
	fmt.Printf("正在验证离线授权文件: %s\n", filePath)
	fmt.Println("")

	// 使用 SDK 验证授权文件
	result, err := lic_sdk.VerifyOfflineAuthFile(filePath)
	if err != nil {
		fmt.Printf("❌ 验证失败: %v\n", err)
		os.Exit(1)
	}

	if !result.Valid {
		fmt.Printf("❌ 授权无效: %s\n", result.Reason)
		os.Exit(1)
	}

	// 输出授权信息
	fmt.Println("✅ 授权验证通过")
	fmt.Println("")
	fmt.Printf("公司/个人名称: %s\n", result.License.Company)
	fmt.Printf("授权码:         %s\n", result.License.LicenseKey)
	fmt.Printf("产品列表:         %v\n", result.License.ProductKeys)
	fmt.Printf("有效期开始:       %s\n", result.License.ValidFrom.Format("2006-01-02"))
	fmt.Printf("有效期结束:       %s\n", result.License.ValidTo.Format("2006-01-02"))
	fmt.Printf("已激活设备:       %d / %d\n", result.License.ActivatedCount, result.License.MaxActivations)
	fmt.Printf("剩余额度:         %d\n", result.License.MaxActivations-result.License.ActivatedCount)
	fmt.Println("")
	fmt.Println("提示: 该授权文件在有效期内始终有效（无需定期联网）")
}

// downloadLicense 从服务器下载离线授权文件
func downloadLicense(token, licenseKey string) {
	serverURL := "http://localhost:8080"
	savePath := fmt.Sprintf("offline-auth-%s.json", licenseKey)

	fmt.Printf("正在从服务器下载授权文件...\n")
	fmt.Printf("  服务器: %s\n", serverURL)
	fmt.Printf("  授权码: %s\n", licenseKey)
	fmt.Println("")

	// 使用 SDK 下载授权文件
	err := lic_sdk.DownloadOfflineAuthFile(serverURL, token, licenseKey, savePath)
	if err != nil {
		fmt.Printf("❌ 下载失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 授权文件已保存: %s\n", savePath)
	fmt.Println("")
	fmt.Println("提示: 使用以下命令验证授权:")
	fmt.Printf("  go run main.go verify %s\n", savePath)
}
