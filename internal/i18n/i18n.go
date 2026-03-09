// i18n 包提供简单的国际化支持。
//
// 支持中文 (zh) 和英文 (en) 两种语言，默认根据系统环境变量
// （LANG / LC_ALL / LANGUAGE）自动检测，也可通过 --lang 命令行参数手动指定。
package i18n

import (
	"fmt"
	"os"
	"strings"
)

// currentLang 当前语言编码，默认为中文
var currentLang = "zh"

// Init 检测系统语言环境变量并设置当前语言
//
// 优先级: LANGUAGE > LC_ALL > LANG。
// 英语前缀设为 en，其他默认为 zh。
func Init() {
	lang := os.Getenv("LANG")
	if l := os.Getenv("LC_ALL"); l != "" {
		lang = l
	}
	if l := os.Getenv("LANGUAGE"); l != "" {
		lang = l
	}
	SetLang(lang)
}

// SetLang 设置当前语言
//
// @param lang 语言编码字符串（如 "en_US.UTF-8" 或 "zh"）
func SetLang(lang string) {
	lang = strings.ToLower(lang)
	switch {
	case strings.HasPrefix(lang, "en"):
		currentLang = "en"
	default:
		currentLang = "zh"
	}
}

// T 根据 key 返回当前语言的翻译字符串
//
// 如果提供了 args 参数，将使用 fmt.Sprintf 格式化翻译模板。
// 对于需要 fmt.Errorf + %w 的场景，应不传 args，将结果传给 fmt.Errorf。
//
// @param key 翻译键名
// @param args 可选的格式化参数
// @return 翻译后的字符串，键不存在时返回键名本身
func T(key string, args ...any) string {
	var m map[string]string
	switch currentLang {
	case "en":
		m = enMessages
	default:
		m = zhMessages
	}
	msg, ok := m[key]
	if !ok {
		msg = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// ExtractGlobalFlags 从命令行参数中提取全局标志
//
// 目前支持的全局标志:
//   - --lang <语言>: 设置界面语言（zh/en）
//
// @param args 原始命令行参数切片
// @return 去除全局标志后的剩余参数
func ExtractGlobalFlags(args []string) []string {
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--lang" && i+1 < len(args) {
			SetLang(args[i+1])
			i++
			continue
		}
		remaining = append(remaining, args[i])
	}
	return remaining
}
