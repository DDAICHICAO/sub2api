package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Protocol baseline: openai/codex rust-v0.167.2, login/src/auth/default_client.rs,
// core/src/client.rs and codex-api/src/endpoint/responses.rs. A forwarding gateway
// must not replace a supported client's OS, build version or capability choices.
func TestCodexProtocolPreservesClientIdentityAcrossTransports(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, client := range []struct{ name, ua, version string }{
		{"codex_cli_rs", "codex_cli_rs/0.156.1 (Windows 11; x86_64) WindowsTerminal", "0.156.1"},
		{"codex-tui", "codex-tui/0.167.2 (Mac OS X 15.0; arm64) iTerm (codex-tui; 0.167.2)", "0.167.2"},
		{"codex_vscode", "codex_vscode/0.167.2 (Windows 11; x86_64) vscode (codex_vscode; 0.167.2)", "0.167.2"},
		{"Codex Desktop", "Codex Desktop/0.167.2 (Mac OS X 15.0; arm64) Terminal", "0.167.2"},
	} {
		for _, transport := range []string{"http", "passthrough", "compact", "ws"} {
			t.Run(client.name+"/"+transport, func(t *testing.T) {
				body := []byte(`{"model":"gpt-6-astra","stream":true,"prompt_cache_key":"cache-key","reasoning":{"effort":"high"},"input":[{"role":"user","content":"hello"}]}`)
				path := "/v1/responses"
				if transport == "compact" {
					path += "/compact"
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
				c.Request.Header.Set("User-Agent", client.ua)
				c.Request.Header.Set("originator", client.name)
				c.Request.Header.Set("session-id", "client-session")
				c.Request.Header.Set("thread-id", "client-thread")
				c.Request.Header.Set("x-client-request-id", "client-thread")
				c.Set("api_key_id", int64(77))
				account := &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
					Credentials: map[string]any{"chatgpt_account_id": "test-account"}}
				svc := &OpenAIGatewayService{cfg: &config.Config{}}
				var h http.Header
				if transport == "ws" {
					var err error
					h, _, err = svc.buildOpenAIWSHeaders(context.Background(), c, account, "test-token",
						OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2}, true, "", "", "cache-key", "", "")
					require.NoError(t, err)
				} else {
					var req *http.Request
					var err error
					if transport == "passthrough" {
						req, err = svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "test-token")
					} else {
						req, err = svc.buildUpstreamRequest(context.Background(), c, account, body, "test-token", true, "cache-key", true)
					}
					require.NoError(t, err)
					h = req.Header
				}
				require.Equal(t, client.ua, h.Get("User-Agent"))
				require.Equal(t, client.name, h.Get("originator"))
				require.Equal(t, client.version, h.Get("version"))
				require.Empty(t, h.Get("x-codex-beta-features"), "absence of advertised features is a valid client choice")
				require.NotEmpty(t, h.Get("session-id"))
				require.NotEmpty(t, h.Get("thread-id"))
				require.NotEqual(t, "client-thread", h.Get("thread-id"), "retain account/user isolation")
				require.Equal(t, h.Get("thread-id"), h.Get("x-client-request-id"), "Codex uses the thread ID as request correlation ID")
				require.Equal(t, "Bearer test-token", h.Get("Authorization"))
				require.Equal(t, "test-account", h.Get("chatgpt-account-id"))
			})
		}
	}
}

func TestCodexProtocolForwardPreservesRequestSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		name := "transformed"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"instructions":"Keep the user's requested behavior.","reasoning":{"effort":"high","summary":"auto"},"tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{}}}],"input":[{"role":"user","content":[{"type":"input_text","text":"continue"}]},{"type":"function_call","call_id":"fc_test","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"fc_test","output":"exact tool result"}]}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
			const ua = "codex_cli_rs/0.156.1 (Windows 11; x86_64) WindowsTerminal"
			c.Request.Header.Set("User-Agent", ua)
			c.Request.Header.Set("originator", "codex_cli_rs")
			c.Request.Header.Set("thread-id", "thread-1")
			c.Request.Header.Set("x-client-request-id", "thread-1")
			c.Set("api_key_id", int64(77))
			upstream := &httpUpstreamRecorder{resp: openAICompatSSECompletedResponse("resp_protocol", "gpt-5.4")}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 123, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
				Credentials: map[string]any{"access_token": "test-token", "chatgpt_account_id": "test-account"},
				Extra:       map[string]any{"openai_passthrough": passthrough}, Concurrency: 1, Status: StatusActive, Schedulable: true, RateMultiplier: f64p(1)}
			result, err := svc.Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, upstream.requests, 1)
			var original, forwarded map[string]any
			require.NoError(t, json.Unmarshal(body, &original))
			require.NoError(t, json.Unmarshal(upstream.lastBody, &forwarded))
			for _, field := range []string{"model", "instructions", "reasoning", "tools", "input"} {
				require.Equal(t, original[field], forwarded[field], field)
			}
			require.Equal(t, ua, upstream.lastReq.UserAgent())
			require.Empty(t, upstream.lastReq.Header.Get("x-codex-beta-features"))
			require.NotEqual(t, "thread-1", upstream.lastReq.Header.Get("thread-id"))
			require.Equal(t, upstream.lastReq.Header.Get("thread-id"), upstream.lastReq.Header.Get("x-client-request-id"))
		})
	}
}

func TestCodexProtocolPreservesThreadRequestCorrelation(t *testing.T) {
	account := &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "test-account"}}
	h := make(http.Header)
	h.Set("thread-id", "same-thread")
	h.Set("x-client-request-id", "same-thread")
	applyCodexAccountIdentityHeaders(h, account, 77)
	require.Equal(t, h.Get("thread-id"), h.Get("x-client-request-id"))
	metadata := map[string]any{"thread_id": "same-thread", "x-client-request-id": "same-thread"}
	require.True(t, applyCodexAccountIdentityFields(metadata, account, 77))
	require.Equal(t, metadata["thread_id"], metadata["x-client-request-id"])
	require.Equal(t, h.Get("thread-id"), metadata["thread_id"])
}

func TestCodexProtocolRejectsInvalidOrObsoleteClientVersion(t *testing.T) {
	for _, ua := range []string{"codex_cli_rs/bogus (Windows)", "codex_cli_rs/0.125.0 (Windows)", "codex_cli_rs/0.167.2\r\nX-Injected: yes"} {
		h := make(http.Header)
		h.Set("User-Agent", ua)
		h.Set("originator", "codex_cli_rs")
		h.Set("version", "999.0.0")
		enforceCodexIdentityHeaders(h)
		require.Equal(t, codexCLIUserAgent, h.Get("User-Agent"))
		require.Equal(t, codexCLIVersion, h.Get("version"))
	}
}

func TestCodexProtocolDoesNotRewriteSupportedVersionToSyncedVersion(t *testing.T) {
	SetCodexCanonicalUserAgentResolver(func() string { return "codex_cli_rs/0.200.1" + codexCLIUserAgentSuffix })
	t.Cleanup(func() { SetCodexCanonicalUserAgentResolver(nil) })
	h := make(http.Header)
	const ua = "codex_cli_rs/0.156.1 (Windows 11; x86_64) WindowsTerminal"
	h.Set("User-Agent", ua)
	h.Set("originator", "codex_cli_rs")
	h.Set("version", "0.167.2") // a stale proxy header must not contradict the actual client build
	enforceCodexIdentityHeaders(h)
	require.Equal(t, ua, h.Get("User-Agent"))
	require.Equal(t, "0.156.1", h.Get("version"))
}
