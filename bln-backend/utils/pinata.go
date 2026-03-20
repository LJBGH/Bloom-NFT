package utils

import (
	"bloom-nft/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// uploadToPinata 上传 io.Reader 内容到 Pinata
func UploadToPinata(filename string, fileData io.Reader) (string, error) {
	jwt := config.AppConfig.IpfsPinana.Jwt
	if jwt == "" {
		return "", fmt.Errorf("IPFS_PINATA.JWT not set in config.yml (or config not loaded)")
	}

	// 创建 multipart 表单
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(part, fileData)
	if err != nil {
		return "", fmt.Errorf("failed to copy file data: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := config.AppConfig.IpfsPinana.UploadUrl + "pinFileToIPFS"

	// 发送请求
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pinata API error: status=%d, body=%s", resp.StatusCode, string(responseBody))
	}

	// 解析返回的 CID（简单方式：假设返回 JSON）
	// 实际建议用 struct unmarshal，这里简化
	// 示例响应: {"IpfsHash":"QmXyZ...","PinSize":12345,"Timestamp":"..."}
	// 我们提取 IpfsHash

	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse Pinata response: %w", err)
	}

	cid, ok := result["IpfsHash"].(string)
	if !ok {
		return "", fmt.Errorf("invalid Pinata response: missing IpfsHash")
	}

	return cid, nil
}

// UploadJSONToPinata 上传任意 JSON-serializable 数据到 Pinata
func UploadJSONToPinata(data interface{}) (string, error) {
	jwt := config.AppConfig.IpfsPinana.Jwt
	if jwt == "" {
		return "", fmt.Errorf("IPFS_PINATA.JWT not set in config.yml")
	}

	// 序列化为 JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// 创建请求 body（纯 JSON）
	body := bytes.NewBuffer(jsonBytes)

	// 使用 Pinata 的 pinJSONToIPFS 接口
	url := config.AppConfig.IpfsPinana.UploadUrl + "pinJSONToIPFS" // 固定地址

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pinata API error: status=%d, body=%s", resp.StatusCode, string(responseBody))
	}

	var result struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse Pinata JSON response: %w", err)
	}

	if result.IpfsHash == "" {
		return "", fmt.Errorf("invalid Pinata response: missing IpfsHash")
	}

	return result.IpfsHash, nil
}
