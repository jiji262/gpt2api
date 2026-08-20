package gateway

import (
	"fmt"
	"strings"
)

// 图像请求的取值上限与默认值。
const (
	maxImagesPerRequest = 4 // IMG2 终稿单轮稳定产出 1-4 张
	defaultImageSize    = "1024x1024"
	defaultImageModel   = "gpt-image-2"
)

// validateImageRequest 按与 chat 相同的三档口径检查图像参数。
//
// 返回非空 param 表示硬拒。判定同样是"语义有值":客户端惯常带上
// background:"auto" / output_format:"png" / quality:"auto" 这类默认值,
// 按 presence 判断会把正常请求全拒掉。
//
// 上游是 chatgpt.com 的网页出图,尺寸、质量、透明背景、输出格式、审核档位
// 全部由其自身策略决定,网关没有任何下发通道 —— 所以只能明确报错,
// 不能像此前那样只落库不下发,让用户以为参数生效了。
func validateImageRequest(r *ImageGenRequest) (param, why string) {
	if strings.TrimSpace(r.Prompt) == "" {
		return "prompt", "不能为空"
	}
	if r.N != 0 && (r.N < 1 || r.N > maxImagesPerRequest) {
		return "n", fmt.Sprintf("取值范围是 1-%d(此前超出会被静默钳位,现在明确拒绝)", maxImagesPerRequest)
	}
	if s := strings.ToLower(strings.TrimSpace(r.Size)); s != "" && s != "auto" && s != defaultImageSize {
		return "size", fmt.Sprintf("上游只产出 %s,尺寸由其自身策略决定,无法指定 %q", defaultImageSize, r.Size)
	}
	if v := lower(r.Quality); v != "" && v != "auto" {
		return "quality", "上游不接受质量档位,输出质量由其自身策略决定"
	}
	if v := lower(r.Style); v != "" && v != "natural" {
		return "style", "上游不接受 style(该参数属于 dall-e-3)"
	}
	if v := lower(r.ResponseFormat); v != "" && v != "url" && v != "b64_json" {
		return "response_format", fmt.Sprintf("只支持 url / b64_json,不认 %q", r.ResponseFormat)
	}
	if v := lower(r.Background); v != "" && v != "auto" && v != "opaque" {
		return "background", "上游只产出不透明背景,做不到透明"
	}
	if v := lower(r.OutputFormat); v != "" && v != "png" {
		return "output_format", "上游只产出 PNG"
	}
	if r.OutputCompression != nil {
		return "output_compression", "仅对 webp / jpeg 有意义,上游只产出 PNG"
	}
	if v := lower(r.Moderation); v != "" && v != "auto" {
		return "moderation", "审核档位由上游自身策略决定,不接受外部配置"
	}
	if r.PartialImages != nil && *r.PartialImages > 0 {
		return "partial_images", "上游不提供渐进式预览帧"
	}
	if r.Stream {
		return "stream", "图像流式(partial_images)依赖上游的渐进帧,本网关拿不到"
	}
	if v := lower(r.InputFidelity); v != "" && v != "low" {
		return "input_fidelity", "上游不接受参考图保真档位"
	}
	return "", ""
}

// validateEditExtras 检查 /v1/images/edits 特有的字段。
//
// mask 必须硬拒:上游没有 inpainting 通道,此前把 mask 当成普通参考图
// 塞进同一个数组,既丢掉了"只改遮罩区域"的语义,又白占一个参考图配额,
// 用户看到的是"图被整体重画了"而不是任何报错。
func validateEditExtras(hasMask bool, inputFidelity string) (param, why string) {
	if hasMask {
		return "mask", "上游没有局部重绘(inpainting)通道,无法只修改遮罩区域。" +
			"请把想要的整体效果写进 prompt,并把原图作为 image 参考图传入"
	}
	if v := lower(inputFidelity); v != "" && v != "low" {
		return "input_fidelity", "上游不接受参考图保真档位"
	}
	return "", ""
}

func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// applyImageDefaults 补齐默认值。必须在 validateImageRequest 之后调用,
// 否则会把用户显式传的越界值改写成合法值,校验就形同虚设。
func applyImageDefaults(r *ImageGenRequest) {
	if r.Model == "" {
		r.Model = defaultImageModel
	}
	if r.N <= 0 {
		r.N = 1
	}
	if r.Size == "" || lower(r.Size) == "auto" {
		r.Size = defaultImageSize
	}
	// 归一成小写:校验用 lower() 放行,而下游是 == "b64_json" 精确比较。
	// 不归一的话 "B64_JSON" 会通过校验然后静默返回 url。
	r.ResponseFormat = lower(r.ResponseFormat)
	if r.ResponseFormat == "" {
		r.ResponseFormat = "url"
	}
	if r.Background == "" {
		r.Background = "auto"
	}
	if r.OutputFormat == "" {
		r.OutputFormat = "png"
	}
	if r.Quality == "" {
		r.Quality = "auto"
	}
}

// newImageGenResponse 建一个已回显请求参数的空响应壳。
//
// 刻意不带 usage:官方 gpt-image-1 响应有 usage.input_tokens / output_tokens,
// 但本网关上游不返回任何 token 计数。编一个数字比缺字段更糟——
// 成本核算侧车会拿它当真。
func newImageGenResponse(r *ImageGenRequest, created int64, taskID string) ImageGenResponse {
	applyImageDefaults(r)
	return ImageGenResponse{
		Created:      created,
		TaskID:       taskID,
		Size:         r.Size,
		Quality:      r.Quality,
		Background:   r.Background,
		OutputFormat: r.OutputFormat,
		Data:         []ImageGenData{},
	}
}
