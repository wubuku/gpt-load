# Key Model Override

## Overview

Key-based model override allows specific API keys to automatically route requests to a different model than what the client requested or what group redirect rules specify. This is useful when you want certain keys to use different model capabilities without changing client configuration.

## Configuration

Create a `key-model-override.json` file in the same directory as the `gpt-load` executable:

```json
{
  "sk-key-prefix-1": "gpt-4o",
  "sk-key-prefix-2": "gpt-4o-mini"
}
```

| Aspect | Detail |
|--------|--------|
| Matching | Key prefix (`strings.HasPrefix`) |
| Loading | Startup only (no hot-reload) |
| Priority | Key override > Group ModelRedirectMap |
| Log level | Debug (`Key model override applied`) |

## Priority Order

1. **Key override** (highest priority) - based on selected API key
2. **Group redirect** - based on `ModelRedirectMap`

## How It Works

### OpenAI Format (JSON body)

When a request comes with a JSON body containing a `model` field:

1. Request body is unmarshaled once
2. Key override is checked first (highest priority)
3. Group redirect is checked second (if no key override matched)
4. Result is marshaled once and returned

### Gemini Native Format (URL path)

When a request comes with a Gemini native URL path like `/v1beta/models/gemini-pro:generateContent`:

1. URL path is parsed to extract the model name
2. Key override is checked first - modifies `req.URL.Path` directly
3. Group redirect is checked second (if no key override matched)
4. Body is returned unchanged (model is in URL, not body)

### OpenAI-Compatible Format (Gemini)

When a request comes with a Gemini channel but uses OpenAI format (`v1beta/openai` path):

- The request is handled by `BaseChannel.ApplyModelRedirect` with JSON body modification

## Implementation

### Files

| File | Purpose |
|------|---------|
| `internal/utils/key_override.go` | Shared utility for loading config and `GetOverrideModel` |
| `internal/channel/channel.go` | Interface definition with `apiKey` parameter |
| `internal/channel/base_channel.go` | Base implementation - applies key override after group redirect |
| `internal/channel/gemini_channel.go` | Gemini-specific override logic - modifies URL path |
| `internal/proxy/server.go` | Caller site - passes `apiKey` to `ApplyModelRedirect` |

### Request Flow

```
proxy/server.go:
  SelectKey → ApplyModelRedirect(req, bodyBytes, group, apiKey)
                      ↓
              Single json.Unmarshal
                      ↓
              Key Override (if matches) → modifies body or URL path
              Group Redirect (if no key override) → modifies body or URL path
                      ↓
              Single json.Marshal (or body unchanged for Gemini URL path)
```

### Logging

When key override is applied, debug logs are emitted:

```
[Debug] Key model override applied {
    "group": "my-group",
    "original_model": "gpt-4o",
    "target_model": "gpt-4o-mini",
    "channel": "json_body"  // or "gemini_native"
}
```

Enable debug logs by setting `LOG_LEVEL=debug`.

---

## History / Change Log

### 2026-04-19: Refactor to Single Parse Cycle

**Problem**: `server.go` had hack code duplicating JSON parse/serialize:
```
SelectKey → ApplyModelRedirect → [hack code] → ModifyRequest
                     ↓
             json.Unmarshal + json.Marshal
                     ↓
             json.Unmarshal + json.Marshal  ← repeated! inefficient
```

**Solution**: Move key override into `ApplyModelRedirect`, pass `apiKey` through the call chain.

**Key decisions**:
- Shared utility in `internal/utils/key_override.go` (avoids circular deps between `proxy` and `channel`)
- Gemini native format modifies URL path (model not in body)
- Key override takes precedence over group redirect (both checked in single unmarshal)

**Files changed**: 5 files - utils/key_override.go, channel/channel.go, channel/base_channel.go, channel/gemini_channel.go, proxy/server.go

### Before

```go
// server.go - interface without apiKey
finalBodyBytes, err := channelHandler.ApplyModelRedirect(req, bodyBytes, group)

// server.go - separate hack code after ApplyModelRedirect
if overrideModel := getOverrideModel(apiKey.KeyValue); overrideModel != "" {
    json.Unmarshal(finalBodyBytes, &requestData)  // double parse!
    requestData["model"] = overrideModel
    json.Marshal(requestData)  // double serialize!
}
```

### After

```go
// server.go - single call with apiKey
finalBodyBytes, err := channelHandler.ApplyModelRedirect(req, bodyBytes, group, apiKey)

// key override handled inside ApplyModelRedirect - single parse/serialize
```

## Build

```bash
# Build frontend and proxy
cd web && npm install && npm run build && cd ..
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o gpt-load .
```

## Verification

- [x] `go build ./...` succeeds
- [x] OpenAI channel: key override works (modifies body)
- [x] Gemini channel: key override works (modifies URL path)
- [x] Anthropic channel: key override works (modifies body via base channel)
- [x] Group redirect + key override: key override takes priority
- [x] Single JSON parse in flow (no double parse/serialize)
- [x] Debug log shows `"Key model override applied"` when triggered
