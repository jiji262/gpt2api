package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/jiji262/gpt2api/pkg/logger"
	"github.com/jiji262/gpt2api/pkg/oaierr"
	"github.com/jiji262/gpt2api/pkg/resp"
)

// Recover 捕获 panic,写入日志并返回 500。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.L().Error("panic recovered",
					zap.Any("err", r),
					zap.ByteString("stack", debug.Stack()),
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", getString(c, "request_id")),
				)
				// /v1 下的 panic 也必须回 OpenAI 错误信封。
				// 回内部的 {"code":50000,...} 形状会让 openai-python 抛
				// 解析异常而不是 APIStatusError,调用方看到的是一个
				// 与真实原因毫无关系的 JSONDecodeError。
				if strings.HasPrefix(c.Request.URL.Path, "/v1/") {
					oaierr.Write(c, http.StatusInternalServerError, "internal_error", "",
						"服务内部错误,请稍后重试(request_id: "+getString(c, "request_id")+")")
					return
				}
				resp.Internal(c, "internal server error")
			}
		}()
		c.Next()
	}
}
