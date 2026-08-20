// localstore.go —— 生图结果字节的本地落盘。
//
// 每个任务的所有 idx 放在一个子目录:
//
//	{root}/{task_id}/{idx}.{ext}
//
// root 默认 "./data/images"(相对进程 cwd,在容器里即 /app/data/images)。
//
// 为什么需要它?参见 migration 20260423000005 的注释 —— 主要是为了解决
// conversation 被 hide 之后 sediment attachment 端点 404 的问题,把图片
// 字节跟上游解耦。

package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultLocalRoot 默认本地存储根目录(相对进程 cwd)。
const DefaultLocalRoot = "./data/images"

// extForContentType 根据 content-type 返回扩展名(含点号)。
// 未知类型落到 .bin,仍能 serve(由代理层用 http.DetectContentType 兜底)。
func extForContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	switch ct {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".bin"
	}
}

// LocalPath 组装单张图的本地路径。idx 从 0 开始。
func LocalPath(root, taskID string, idx int, contentType string) string {
	if root == "" {
		root = DefaultLocalRoot
	}
	return filepath.Join(root, taskID, fmt.Sprintf("%d%s", idx, extForContentType(contentType)))
}

// ExistsAny 在 root/{task_id}/{idx}.* 里找到第一个存在的文件(任意扩展名)。
// 返回 (path, exists)。用于 proxy 层只知道 task_id+idx,不存 content-type 时的查找兜底。
// 但本项目 content-type 会存 DB,所以主路径是 LocalPath 直接拼出来,这个函数作为异常恢复用。
func ExistsAny(root, taskID string, idx int) (string, bool) {
	if root == "" {
		root = DefaultLocalRoot
	}
	dir := filepath.Join(root, taskID)
	for _, ext := range []string{".png", ".jpg", ".webp", ".gif", ".bin"} {
		p := filepath.Join(dir, fmt.Sprintf("%d%s", idx, ext))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

// SaveImage 把字节写到磁盘。目录不存在会自动创建。
// 写文件用"临时文件 + rename"保证原子(避免 proxy 读到半写入文件)。
func SaveImage(root, taskID string, idx int, contentType string, data []byte) (string, error) {
	dst := LocalPath(root, taskID, idx, contentType)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", fmt.Errorf("rename %s -> %s: %w", tmp, dst, err)
	}
	return dst, nil
}
