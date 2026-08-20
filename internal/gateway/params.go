package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/pkg/oaierr"
)

// 消息角色。developer 是 system 的新名字(官方 2024-12 起推荐),上游只认 system。
const (
	roleSystem    = "system"
	roleDeveloper = "developer"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
	roleFunction  = "function"
)

// 上游是 chatgpt.com 网页版,报错文案统一带上这句,让用户知道不是自己参数写错了。
const upstreamCaveat = "本网关上游为 chatgpt.com 网页版,不提供该能力"

// chatParamVerdict 是参数校验结论。
//
// RejectParam 非空表示硬拒;Ignored 是软忽略清单,会以响应头回给调用方。
type chatParamVerdict struct {
	RejectParam   string
	RejectMessage string
	Ignored       []string
}

// validateChatRequest 按三档策略检查请求,并就地完成别名映射。
//
// 判定用的是"语义有值",不是"字段存在":真实客户端会无条件带上默认值
// (openai-node 总发 "tools":[]、LangChain 总发 "stop":null),
// 按 presence 判断会把完全正常的请求全部拒掉。
func validateChatRequest(r *ChatCompletionsRequest) chatParamVerdict {
	return validateChatRequestWithVision(r, false)
}

// validateChatRequestWithVision 在开启 vision 时放行 image_url part。
// 其余判定完全一致 —— 两套口径分叉是回归的温床,所以只加这一个开关。
func validateChatRequestWithVision(r *ChatCompletionsRequest, vision bool) chatParamVerdict {
	var v chatParamVerdict

	// ---- Alias 档:换个名字就能生效 ----
	if r.MaxTokens <= 0 && r.MaxCompletionTokens != nil && *r.MaxCompletionTokens > 0 {
		r.MaxTokens = *r.MaxCompletionTokens
	}

	// ---- 硬拒档:上游做不到,且用户明确要了该行为 ----
	if p, why := firstUnsupported(r); p != "" {
		v.RejectParam = p
		v.RejectMessage = why
		return v
	}
	if p, why := unsupportedMessageShape(r.Messages, vision); p != "" {
		v.RejectParam = p
		v.RejectMessage = why
		return v
	}

	// ---- 软忽略档:不影响正确性,只是不生效 ----
	v.Ignored = ignoredParams(r)
	return v
}

// firstUnsupported 返回第一个"用户明确要了但上游做不到"的参数。
// 顺序按客户端最可能踩到的先后排,让报错指向最关键的那个。
func firstUnsupported(r *ChatCompletionsRequest) (param, why string) {
	if rawNonEmptyArray(r.Tools) {
		return "tools", "上游没有工具调用通道,工具定义无法下发,模型永远不会返回 tool_calls。" +
			"若只是想让模型输出结构化内容,请在 prompt 里描述格式并自行解析"
	}
	if meaningfulToolChoice(r.ToolChoice) {
		return "tool_choice", "上游没有工具调用通道"
	}
	if rawNonEmptyArray(r.Functions) {
		return "functions", "上游没有工具调用通道(functions 是已弃用的 tools 旧写法)"
	}
	if rawPresent(r.FunctionCall) && !rawEquals(r.FunctionCall, `"none"`) {
		return "function_call", "上游没有工具调用通道"
	}
	if r.ResponseFormat != nil && r.ResponseFormat.Type != "" && r.ResponseFormat.Type != "text" {
		return "response_format", fmt.Sprintf(
			"上游没有 schema 约束通道,%q 无法保证输出格式。请在 prompt 里要求 JSON 并对结果做容错解析",
			r.ResponseFormat.Type)
	}
	if r.N != nil && *r.N > 1 {
		return "n", "上游一次请求只产出一条回复,n>1 做不到。请改为发多次请求"
	}
	if meaningfulStop(r.Stop) {
		return "stop", "上游不接受停止序列,返回内容不会在指定字符串处截断"
	}
	if r.Logprobs != nil && *r.Logprobs {
		return "logprobs", "上游不返回 token 概率"
	}
	if r.TopLogprobs != nil {
		return "top_logprobs", "上游不返回 token 概率"
	}
	if len(r.LogitBias) > 0 {
		return "logit_bias", "上游不接受 token 偏置"
	}
	if r.PresencePenalty != nil && *r.PresencePenalty != 0 {
		return "presence_penalty", "上游不接受采样惩罚项"
	}
	if r.FrequencyPenalty != nil && *r.FrequencyPenalty != 0 {
		return "frequency_penalty", "上游不接受采样惩罚项"
	}
	if r.Seed != nil {
		return "seed", "上游不支持确定性采样,相同 seed 不保证相同输出"
	}
	if m := nonTextModality(r.Modalities); m != "" {
		return "modalities", fmt.Sprintf("上游只产出文本,不支持 %q 模态", m)
	}
	if rawPresent(r.Audio) {
		return "audio", "上游不支持语音输出"
	}
	if rawPresent(r.Prediction) {
		return "prediction", "上游不支持预测式输出(Predicted Outputs)"
	}
	if rawPresent(r.WebSearchOptions) {
		return "web_search_options", "上游的联网检索由其自身策略决定,不接受外部配置"
	}
	return "", ""
}

// unsupportedMessageShape 检查消息角色与 content part 形态。
func unsupportedMessageShape(msgs []RequestMessage, vision bool) (param, why string) {
	for i, m := range msgs {
		switch m.Role {
		case roleSystem, roleDeveloper, roleUser, roleAssistant, "":
			// ok
		case roleTool, roleFunction:
			return fmt.Sprintf("messages[%d].role", i),
				"tool / function 角色的消息依赖工具调用回填,上游没有该通道"
		default:
			return fmt.Sprintf("messages[%d].role", i),
				fmt.Sprintf("未知角色 %q", m.Role)
		}
		if rawNonEmptyArray(m.ToolCalls) {
			return fmt.Sprintf("messages[%d].tool_calls", i), "上游没有工具调用通道"
		}
		if blocked := blockedParts(m.Content.NonTextParts, vision); len(blocked) > 0 {
			return fmt.Sprintf("messages[%d].content", i), fmt.Sprintf(
				"暂不支持 %s 类型的 content part,%s。请只发文本内容",
				strings.Join(blocked, " / "), upstreamCaveat)
		}
	}
	return "", ""
}

// blockedParts 返回仍然做不到的 part 类型。
// 开启 vision 后 image_url 走附件通道,不再算做不到;
// input_audio / file 无论如何都没有对应通道。
func blockedParts(parts []string, vision bool) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if vision && p == "image_url" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ignoredParams 收集"传了但不会生效"的参数,只在值有实际意义时才记,
// 避免把客户端无条件带上的官方默认值也报成忽略,变成纯噪声。
func ignoredParams(r *ChatCompletionsRequest) []string {
	var out []string
	add := func(name string) { out = append(out, name) }

	if r.Temperature != nil && *r.Temperature != 1 {
		add("temperature")
	}
	if r.TopP != nil && *r.TopP != 1 {
		add("top_p")
	}
	if r.ReasoningEffort != "" {
		add("reasoning_effort")
	}
	if r.Verbosity != "" {
		add("verbosity")
	}
	if r.ServiceTier != "" && r.ServiceTier != "auto" {
		add("service_tier")
	}
	if r.Store != nil && *r.Store {
		add("store")
	}
	if len(r.Metadata) > 0 {
		add("metadata")
	}
	if r.SafetyIdentifier != "" {
		add("safety_identifier")
	}
	if r.PromptCacheKey != "" {
		add("prompt_cache_key")
	}
	return out
}

// ---------------------------------------------------------------- 取值判定

// rawPresent 判断 RawMessage 是否携带了非 null 的值。
// 注意 "null" 会被解成 4 字节的 RawMessage,不是 nil。
func rawPresent(r json.RawMessage) bool {
	s := strings.TrimSpace(string(r))
	return s != "" && s != "null"
}

func rawEquals(r json.RawMessage, want string) bool {
	return strings.TrimSpace(string(r)) == want
}

// rawNonEmptyArray 判断是不是"非空数组"。[] 与 null 都算没传。
func rawNonEmptyArray(r json.RawMessage) bool {
	if !rawPresent(r) {
		return false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(r, &arr); err != nil {
		// 不是数组(客户端传错类型),交给上层当作"有值"处理,报错总比静默好。
		return true
	}
	return len(arr) > 0
}

// meaningfulToolChoice:none 表示明确不用工具,auto 是默认值,两者都无需拒绝。
func meaningfulToolChoice(r json.RawMessage) bool {
	if !rawPresent(r) {
		return false
	}
	return !rawEquals(r, `"none"`) && !rawEquals(r, `"auto"`)
}

// meaningfulStop:stop 可以是 string 或 []string,空串与空数组都算没传。
func meaningfulStop(r json.RawMessage) bool {
	if !rawPresent(r) {
		return false
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		return s != ""
	}
	return rawNonEmptyArray(r)
}

// nonTextModality 返回第一个非 text 模态。modalities:["text"] 是默认值。
func nonTextModality(ms []string) string {
	for _, m := range ms {
		if m != "" && m != "text" {
			return m
		}
	}
	return ""
}

// ---------------------------------------------------------------- HTTP 输出

// writeUnsupportedParam 输出统一的 400 unsupported_parameter,param 指向具体字段。
func writeUnsupportedParam(c *gin.Context, param, why string) {
	oaierr.Write(c, http.StatusBadRequest, oaierr.CodeUnsupportedParameter, param,
		fmt.Sprintf("参数 %s 暂不支持:%s(本网关上游为 chatgpt.com 网页版)", param, why))
}

// setIgnoredParamsHeader 把软忽略清单回给调用方。
// 用响应头而不是响应体,是为了不污染 OpenAI 协议规定的响应结构。
func setIgnoredParamsHeader(c *gin.Context, ignored []string) {
	if len(ignored) == 0 {
		return
	}
	c.Header("X-Gateway-Ignored-Params", strings.Join(ignored, ","))
}
