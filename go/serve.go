package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type apiResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Server  string `json:"server,omitempty"`
	Report  string `json:"report,omitempty"`
}

func reportDir(cfg *Config) string {
	configPath := findConfigPath()
	dir := filepath.Dir(configPath)
	return filepath.Join(dir, "reports")
}

func todayReportPath(cfg *Config) string {
	return filepath.Join(reportDir(cfg), "today.json")
}

func dateReportPath(cfg *Config, date string) string {
	return filepath.Join(reportDir(cfg), date+".json")
}

func loadReportFile(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func startAPIServer(cfg *Config) {
	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.APIPort)
	os.MkdirAll(reportDir(cfg), 0755)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" || key != cfg.Server.APIKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(apiResponse{Status: "error", Message: "invalid or missing api key"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse{
			Status: "ok",
			Server: cfg.Server.Name,
		})
	})

	mux.HandleFunc("/api/v1/report", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" || key != cfg.Server.APIKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(apiResponse{Status: "error", Message: "invalid or missing api key"})
			return
		}

		// 优先读取缓存的 today.json
		date := strings.TrimSpace(r.URL.Query().Get("date"))
		var reportPath string
		if date != "" {
			reportPath = dateReportPath(cfg, date)
		} else {
			reportPath = todayReportPath(cfg)
		}

		if data, err := loadReportFile(reportPath); err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
			return
		}

		// 没有缓存文件则实时生成
		result := buildJSONReport(cfg)

		// 尝试保存为 today.json 供下次使用
		if data, err := json.Marshal(result); err == nil {
			os.WriteFile(todayReportPath(cfg), data, 0644)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("API 服务启动失败: %v", err)
	}

	fmt.Printf("API 服务已启动: http://%s\n", addr)
	fmt.Printf("API Key: %s\n", cfg.Server.APIKey)
	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("API 服务异常退出: %v", err)
	}
}
