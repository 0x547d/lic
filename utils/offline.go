package utils

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/0x547d/lic/models"
)

// BuildOfflineRequest 客户端生成离线激活请求文件内容
func BuildOfflineRequest(licenseKey, deviceFingerprint, clientVersion string, privateKeyPEM string) (*models.OfflineActivationRequest, error) {
	req := &models.OfflineActivationRequest{
		Version:           "1.0",
		LicenseKey:        licenseKey,
		DeviceFingerprint: deviceFingerprint,
		ClientVersion:     clientVersion,
		Timestamp:         time.Now().Unix(),
	}

	// 用客户端私钥签名请求内容（可选，增强安全性）
	if privateKeyPEM != "" {
		sig, err := signData(getRequestSignData(req), privateKeyPEM)
		if err == nil {
			req.Signature = sig
		}
	}

	return req, nil
}

// BuildOfflineResponse 服务端生成离线激活响应文件内容
func BuildOfflineResponse(license *models.License, deviceFingerprint string) (*models.OfflineActivationResponse, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("server private key not initialized")
	}

	resp := &models.OfflineActivationResponse{
		Version:           "1.0",
		LicenseKey:        license.LicenseKey,
		DeviceFingerprint: deviceFingerprint,
		ValidFrom:         license.ValidFrom,
		ValidTo:           license.ValidTo,
		IssuedAt:          time.Now(),
		Certificate:       GetPublicKeyPEM(),
	}

	// 用服务端私钥签名响应内容
	sig, err := signResponseData(resp, privateKey)
	if err != nil {
		return nil, err
	}
	resp.Signature = sig
	return resp, nil
}

// VerifyOfflineResponse 客户端验证离线激活响应文件
func VerifyOfflineResponse(resp *models.OfflineActivationResponse, serverPublicKeyPEM string) (bool, error) {
	// 验证时间有效性
	now := time.Now()
	if now.Before(resp.ValidFrom) || now.After(resp.ValidTo) {
		return false, fmt.Errorf("license time range invalid")
	}

	// 验证服务端签名
	pubKey, err := parsePublicKey(serverPublicKeyPEM)
	if err != nil {
		return false, fmt.Errorf("invalid server public key: %w", err)
	}

	return verifyResponseSignature(resp, pubKey)
}

// getRequestSignData 获取请求签名数据
func getRequestSignData(req *models.OfflineActivationRequest) string {
	return fmt.Sprintf("%s|%s|%s|%d",
		req.LicenseKey, req.DeviceFingerprint, req.ClientVersion, req.Timestamp)
}

// getResponseSignData 获取响应签名数据
func getResponseSignData(resp *models.OfflineActivationResponse) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s",
		resp.Version, resp.LicenseKey, resp.DeviceFingerprint,
		resp.ValidFrom.Format(time.RFC3339), resp.ValidTo.Format(time.RFC3339))
}

// signData 使用 RSA 私钥签名数据
func signData(data string, privateKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(data))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signature), nil
}

// signResponseData 签名响应数据
func signResponseData(resp *models.OfflineActivationResponse, key *rsa.PrivateKey) (string, error) {
	data := getResponseSignData(resp)
	hash := sha256.Sum256([]byte(data))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signature), nil
}

// verifyResponseSignature 验证响应签名
func verifyResponseSignature(resp *models.OfflineActivationResponse, pubKey *rsa.PublicKey) (bool, error) {
	data := getResponseSignData(resp)
	hash := sha256.Sum256([]byte(data))
	sig, err := hex.DecodeString(resp.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature encoding: %w", err)
	}
	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sig)
	return err == nil, err
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

// DecodeRequestFile 解析客户端上传的离线激活请求文件（JSON 格式）
func DecodeRequestFile(jsonData []byte) (*models.OfflineActivationRequest, error) {
	var req models.OfflineActivationRequest
	err := json.Unmarshal(jsonData, &req)
	return &req, err
}

// DecodeResponseFile 解析服务端下发的离线激活响应文件（JSON 格式）
func DecodeResponseFile(jsonData []byte) (*models.OfflineActivationResponse, error) {
	var resp models.OfflineActivationResponse
	err := json.Unmarshal(jsonData, &resp)
	return &resp, err
}

// EncodeRequestFile 将请求结构体编码为 JSON 文件内容
func EncodeRequestFile(req *models.OfflineActivationRequest) ([]byte, error) {
	return json.MarshalIndent(req, "", "  ")
}

// EncodeResponseFile 将响应结构体编码为 JSON 文件内容
func EncodeResponseFile(resp *models.OfflineActivationResponse) ([]byte, error) {
	return json.MarshalIndent(resp, "", "  ")
}
