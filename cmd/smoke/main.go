// Command smoke 起一个不带 DB/Redis 的最小网关,用于手工冒烟测试路由与协议形状。
// 仅用于本地验证,不参与部署。
package main

import (
	"log"
	"net/http"

	"github.com/jiji262/gpt2api/internal/config"
	"github.com/jiji262/gpt2api/internal/gateway"
	"github.com/jiji262/gpt2api/internal/server"
)

func main() {
	cfg := &config.Config{}
	cfg.App.Env = "test"
	cfg.Security.CORSOrigins = []string{"*"}
	r := server.New(&server.Deps{Config: cfg, GatewayH: &gateway.Handler{}})
	log.Println("smoke server on :18080")
	_ = http.ListenAndServe("127.0.0.1:18080", r)
}
