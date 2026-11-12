package service

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// openAINativeCompactionV2Key 标记本请求是原生 remote compaction v2
// （裸 /responses + stream:true + compaction_trigger），由 handler 在判定后
// 写入，供上游请求构造时补注协商头。
const openAINativeCompactionV2Key = "openai_native_compaction_v2"

const openAIRemoteCompactionV2Feature = "remote_compaction_v2"

// MarkOpenAINativeCompactionV2 由 handler 在识别出原生 v2 压缩请求时调用。
func MarkOpenAINativeCompactionV2(c *gin.Context) {
	if c != nil {
		c.Set(openAINativeCompactionV2Key, true)
	}
}

// NormalizeCompactionTriggerInputOrder keeps a single compaction trigger as
// the final Responses input item, as required by the upstream v2 wire format.
func NormalizeCompactionTriggerInputOrder(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}
	if !compactionTriggerNeedsDecode(body) {
		return body, false, nil
	}
	var payload map[string]any
	if err := decodeOpenAIJSONUseNumber(body, &payload); err != nil {
		return body, false, err
	}
	input, ok := payload["input"].([]any)
	if !ok || len(input) == 0 {
		return body, false, nil
	}
	triggerCount := 0
	normalized := make([]any, 0, len(input))
	for _, raw := range input {
		item, itemOK := raw.(map[string]any)
		if itemOK && item["type"] == "compaction_trigger" {
			triggerCount++
			continue
		}
		normalized = append(normalized, raw)
	}
	if triggerCount == 0 {
		return body, false, nil
	}
	if triggerCount == 1 {
		if last, ok := input[len(input)-1].(map[string]any); ok && last["type"] == "compaction_trigger" {
			return body, false, nil
		}
	}
	normalized = append(normalized, map[string]any{"type": "compaction_trigger"})
	payload["input"] = normalized
	encoded, err := marshalOpenAIUpstreamJSON(payload)
	if err != nil {
		return body, false, err
	}
	return encoded, true, nil
}

// compactionTriggerNeedsDecode lets bodies that cannot hold a trigger skip the
// full decode: the type string appears literally unless a \u escape spells it.
// Such a body must still be a valid JSON object, so decode errors keep
// surfacing from the original path.
func compactionTriggerNeedsDecode(body []byte) bool {
	if bytes.Contains(body, []byte("compaction_trigger")) || bytes.Contains(body, []byte(`\u`)) {
		return true
	}
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	return len(trimmed) == 0 || trimmed[0] != '{' || !gjson.ValidBytes(body)
}

func isOpenAINativeCompactionV2(c *gin.Context) bool {
	if c == nil {
		return false
	}
	return c.GetBool(openAINativeCompactionV2Key)
}

// IsOpenAINativeCompactionV2 reports whether the handler identified this
// request as the native remote compaction v2 wire. It exposes only the
// request-scoped boolean marker; no request payload is retained.
func IsOpenAINativeCompactionV2(c *gin.Context) bool {
	return isOpenAINativeCompactionV2(c)
}

// ensureOpenAIRemoteCompactionV2BetaFeature 确保出站 x-codex-beta-features
// 头包含 remote_compaction_v2。真实 Codex 发送 compaction_trigger 时总会同时
// 携带该协商头（codex-rs build_model_client_beta_features_header 对该 feature
// 特判 advertise）；上游或下游网关链剥掉它后，请求会在依赖该头做门控的
// 环节被降级（#5586）。这里在原生 v2 请求出站前补齐，使线型与真实 Codex
// 一致。已存在时保持原样，不重复追加。
func ensureOpenAIRemoteCompactionV2BetaFeature(h http.Header) {
	if h == nil {
		return
	}
	tokens := make([]string, 0, 4)
	for _, value := range h.Values("x-codex-beta-features") {
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if token == openAIRemoteCompactionV2Feature {
				return
			}
			tokens = append(tokens, token)
		}
	}
	tokens = append(tokens, openAIRemoteCompactionV2Feature)
	h.Set("x-codex-beta-features", strings.Join(tokens, ","))
}

// hasOpenAICodexBetaFeaturesHeader 报告出站头里是否已存在非空的
// x-codex-beta-features（即客户端自己声明过能力集）。
func hasOpenAICodexBetaFeaturesHeader(h http.Header) bool {
	if h == nil {
		return false
	}
	for _, value := range h.Values("x-codex-beta-features") {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// applyOpenAICodexBetaFeatures preserves the client's advertised capability set,
// including an absent header on supported Codex clients (codex-rs/core/src/client.rs,
// build_responses_headers). A native compaction_trigger proves that v2 is in use
// and still ensures its negotiation token. Requests without a supported Codex
// identity retain the legacy OAuth compatibility default; API-key backends do not.
func applyOpenAICodexBetaFeatures(c *gin.Context, account *Account, h http.Header) {
	if h == nil {
		return
	}
	if isOpenAINativeCompactionV2(c) {
		ensureOpenAIRemoteCompactionV2BetaFeature(h)
		return
	}
	if account == nil || !account.IsOpenAIOAuthLike() {
		return
	}
	if hasOpenAICodexBetaFeaturesHeader(h) {
		return
	}
	// Official clients omit this header when no advertised feature is enabled.
	// Preserve that choice instead of claiming support for remote compaction v2.
	// Check the inbound identity: the outbound fallback is also Codex-shaped.
	if c != nil && c.Request != nil {
		if _, ok := codexClientIdentityFromUA(c.Request.UserAgent()); ok {
			return
		}
	}
	h.Set("x-codex-beta-features", openAIRemoteCompactionV2Feature)
}

// HasCompactionTriggerInInput detects an input item with
// type="compaction_trigger". The handler combines this body signal with the
// request path and stream flag to distinguish the native remote compaction v2
// wire from the legacy /responses/compact bridge.
func HasCompactionTriggerInInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "compaction_trigger" {
			found = true
			return false
		}
		return true
	})
	return found
}
