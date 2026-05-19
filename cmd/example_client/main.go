package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const serverURL = "http://localhost:8080/api/v1"

// ===== 公共结构体 =====

type LoginResp struct {
	Token      string `json:"token"`
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	LicenseKey string `json:"license_key"`
	ExpiresIn  int    `json:"expires_in"`
}

type VerifyResp struct {
	Valid     bool      `json:"valid"`
	Status    string    `json:"status"`
	ValidFrom time.Time `json:"valid_from"`
	ValidTo   time.Time `json:"valid_to"`
}

type ActivateResp struct {
	Message    string    `json:"message"`
	ActivationID uint   `json:"activation_id"`
	ValidTo     time.Time `json:"valid_to"`
}

// ===== 在线激活流程 =====

func onlineFlow() {
	fmt.Println("=== 在线激活流程 ===")

	// 1. 登录获取 Token
	token, licenseKey := login("test", "123456")

	// 2. 验证授权状态
	verify(token, licenseKey)

	// 3. 在线激活（绑定设备指纹）
	activateOnline(token, licenseKey, getDeviceFingerprint())

	// 4. 客户端每次启动：验证授权
	verify(token, licenseKey)
}

func login(username, password string) (string, string) {
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := http.Post(serverURL+"/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var result LoginResp
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("✅ 登录成功，UserID=%d，LicenseKey=%s\n", result.UserID, result.LicenseKey)
	return result.Token, result.LicenseKey
}

func verify(token, licenseKey string) {
	body, _ := json.Marshal(map[string]string{"license_key": licenseKey})
	req, _ := http.NewRequest("POST", serverURL+"/verify", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("⚠️  验证失败（可能离线）：", err)
		return
	}
	defer resp.Body.Close()

	var result VerifyResp
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Valid {
		fmt.Printf("✅ 授权有效，到期时间：%s\n", result.ValidTo.Format("2006-01-02"))
	} else {
		fmt.Printf("❌ 授权无效：%s\n", result.Status)
	}
}

func activateOnline(token, licenseKey, deviceFP string) {
	body, _ := json.Marshal(map[string]string{
		"license_key":        licenseKey,
		"device_fingerprint": deviceFP,
		"client_version":     "1.0.0",
	})
	req, _ := http.NewRequest("POST", serverURL+"/activate", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var result ActivateResp
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("✅ 在线激活成功：%s，到期：%s\n", result.Message, result.ValidTo)
}

// ===== 离线激活流程 =====

func offlineFlow() {
	fmt.Println("\n=== 离线激活流程 ===")

	// [离线客户端] 1. 生成离线激活请求
	licenseKey := "A1B2-C3D4-E5F6-7890" // 从配置或用户输入获取
	deviceFP := getDeviceFingerprint()

	reqBody, _ := json.Marshal(map[string]string{
		"license_key":        licenseKey,
		"device_fingerprint": deviceFP,
		"client_version":     "1.0.0",
	})

	// 这一步在离线客户端上执行，拿到 requestJSON 后手动传到联网机器
	resp, err := http.Post(serverURL+"/offline/request/gen", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Println("⚠️  离线请求生成失败（客户端需在有网环境执行此步，或手动构造请求文件）")
		return
	}
	defer resp.Body.Close()

	var genResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&genResult)
	fmt.Printf("✅ 离线请求已生成，token=%s\n", genResult["request_token"])
	fmt.Printf("   请求文件内容：\n%s\n", genResult["request_file"])
	fmt.Println("   👉 请将上述内容保存到 request.json，传到联网机器处理")

	// [联网机器] 2. 处理离线激活，下载响应文件
	// 实际场景：在联网机器上调用 /api/v1/offline/activate/<token>
	token := genResult["request_token"].(string)
	activateResp, err := http.Post(serverURL+"/offline/activate/"+token, "application/json", nil)
	if err != nil {
		panic(err)
	}
	defer activateResp.Body.Close()

	responseFile, _ := io.ReadAll(activateResp.Body)
	fmt.Printf("✅ 离线响应文件已生成：\n%s\n", string(responseFile))
	fmt.Println("   👉 请将响应文件内容保存到 response.json，传回离线客户端")

	// [离线客户端] 3. 验证离线响应文件
	verifyReq, _ := json.Marshal(map[string]string{
		"response_file": string(responseFile),
	})
	verifyResp, err := http.Post(serverURL+"/offline/verify", "application/json", bytes.NewBuffer(verifyReq))
	if err != nil {
		panic(err)
	}
	defer verifyResp.Body.Close()

	var verifyResult map[string]interface{}
	json.NewDecoder(verifyResp.Body).Decode(&verifyResult)
	if verifyResp.StatusCode == 200 {
		fmt.Printf("✅ 离线授权验证成功！有效至：%v\n", verifyResult["valid_to"])
	} else {
		fmt.Printf("❌ 离线授权验证失败：%v\n", verifyResult["error"])
	}
}

// ===== 工具函数 =====

func getDeviceFingerprint() string {
	// 实际项目中应采集 CPU ID / 主板序列号 / MAC 地址等硬件信息
	// 这里用简单示例代替
	return fmt.Sprintf("DEV-%s", time.Now().Format("20060102"))
}

func main() {
	// 在线激活流程示例
	onlineFlow()

	// 离线激活流程示例
	offlineFlow()
}
