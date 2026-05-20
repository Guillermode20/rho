# `@earendil-works/pi-ai` — Complete Feature Reference

**Version:** 0.75.3  
**Description:** Unified LLM API with automatic model discovery and provider configuration  
**Package:** `@earendil-works/pi-ai` (published under `packages/ai` in the pi monorepo)

---

## 1. Architecture Overview

This package provides a **unified abstraction layer** over many LLM providers. It offers:

- A **unified type system** (`Model`, `Message`, `Context`, `Tool`, etc.)
- An **API registry** where providers register their stream/generate implementations
- A **model registry** with ~500+ pre-configured models across ~30 providers
- **Lazy-loaded provider modules** (only loaded when first used)
- **Streaming event protocol** (`AssistantMessageEventStream`)
- **Environment-based API key resolution**
- **OAuth 2.0 login flows** for Anthropic, GitHub Copilot, and OpenAI Codex
- **Cross-provider message transformation** (converts between provider-specific formats)
- **Context overflow detection**
- **Tool call argument validation** with TypeBox schemas

---

## 2. Core Exports (from `src/index.ts`)

### 2.1 Type Re-exports

| Export | From |
|--------|------|
| `Static` | `typebox` |
| `TSchema` | `typebox` |
| `Type` | `typebox` |

### 2.2 Full Module Re-exports (`export * from ...`)

| Module | Description |
|--------|-------------|
| `api-registry.js` | API provider registration/lookup |
| `env-api-keys.js` | Environment-variable API key resolution |
| `image-models.js` | Image model registry (getImageModel, getImageModels, getImageProviders) |
| `images.js` | Image generation entry point (`generateImages()`) |
| `images-api-registry.js` | Images API provider registration |
| `models.js` | Model registry (getModel, getModels, getProviders, calculateCost, getSupportedThinkingLevels, clampThinkingLevel, modelsAreEqual) |
| `providers/faux.js` | Testing/mock provider (registerFauxProvider) |
| `providers/images/register-builtins.js` | Registers built-in image providers |
| `providers/register-builtins.js` | Registers all built-in chat providers |
| `session-resources.js` | Session resource lifecycle management |
| `stream.js` | High-level streaming API (stream, complete, streamSimple, completeSimple) |
| `types.js` | All TypeScript type definitions |
| `utils/diagnostics.js` | Diagnostics utilities |
| `utils/event-stream.js` | EventStream / AssistantMessageEventStream classes |
| `utils/json-parse.js` | JSON repair and streaming JSON parsing |
| `utils/overflow.js` | Context overflow detection |
| `utils/typebox-helpers.js` | StringEnum helper for TypeBox |
| `utils/validation.js` | Tool call argument validation with TypeBox |

### 2.3 Type-only Re-exports

| Export | Source |
|--------|--------|
| `BedrockOptions`, `BedrockThinkingDisplay` | `providers/amazon-bedrock.js` |
| `AnthropicEffort`, `AnthropicOptions`, `AnthropicThinkingDisplay` | `providers/anthropic.js` |
| `AzureOpenAIResponsesOptions` | `providers/azure-openai-responses.js` |
| `GoogleOptions` | `providers/google.js` |
| `GoogleThinkingLevel` | `providers/google-shared.js` |
| `GoogleVertexOptions` | `providers/google-vertex.js` |
| `MistralOptions` | `providers/mistral.js` |
| `OpenAICodexResponsesOptions`, `OpenAICodexWebSocketDebugStats` | `providers/openai-codex-responses.js` |
| `OpenAICompletionsOptions` | `providers/openai-completions.js` |
| `OpenAIResponsesOptions` | `providers/openai-responses.js` |
| `OAuthAuthInfo`, `OAuthCredentials`, `OAuthLoginCallbacks`, `OAuthPrompt`, `OAuthProvider`, `OAuthProviderId`, `OAuthProviderInfo`, `OAuthProviderInterface`, `OAuthSelectOption`, `OAuthSelectPrompt` | `utils/oauth/types.js` |

---

## 3. Type System (from `src/types.ts`)

### 3.1 API Types

```typescript
type KnownApi =
  | "openai-completions" | "mistral-conversations" | "openai-responses"
  | "azure-openai-responses" | "openai-codex-responses" | "anthropic-messages"
  | "bedrock-converse-stream" | "google-generative-ai" | "google-vertex";

type Api = KnownApi | (string & {});

type KnownImagesApi = "openrouter-images";
type ImagesApi = KnownImagesApi | (string & {});
```

### 3.2 Provider Types

```typescript
type KnownProvider =
  | "amazon-bedrock" | "anthropic" | "google" | "google-vertex" | "openai"
  | "azure-openai-responses" | "openai-codex" | "deepseek" | "github-copilot"
  | "xai" | "groq" | "cerebras" | "openrouter" | "vercel-ai-gateway" | "zai"
  | "mistral" | "minimax" | "minimax-cn" | "moonshotai" | "moonshotai-cn"
  | "huggingface" | "fireworks" | "together" | "opencode" | "opencode-go"
  | "kimi-coding" | "cloudflare-workers-ai" | "cloudflare-ai-gateway"
  | "xiaomi" | "xiaomi-token-plan-cn" | "xiaomi-token-plan-ams" | "xiaomi-token-plan-sgp";

type Provider = KnownProvider | string;
```

### 3.3 Core Data Types

- **`Message`** — Union of `UserMessage`, `AssistantMessage`, `ToolResultMessage`
- **`UserMessage`** — Role `"user"`, string or (TextContent | ImageContent)[] content
- **`AssistantMessage`** — Role `"assistant"`, content blocks, usage, diagnostics, stopReason, responseId, etc.
- **`ToolResultMessage`** — Role `"toolResult"`, toolCallId, toolName, content, isError
- **`Context`** — `{ systemPrompt?, messages, tools? }`
- **`Tool<TParameters>`** — `{ name, description, parameters: TSchema }`
- **`Usage`** — `{ input, output, cacheRead, cacheWrite, totalTokens, cost }`
- **`StopReason`** — `"stop" | "length" | "toolUse" | "error" | "aborted"`

### 3.4 Content Block Types

- **`TextContent`** — `{ type: "text", text, textSignature? }`
- **`ThinkingContent`** — `{ type: "thinking", thinking, thinkingSignature?, redacted? }`
- **`ImageContent`** — `{ type: "image", data (base64), mimeType }`
- **`ToolCall`** — `{ type: "toolCall", id, name, arguments, thoughtSignature? }`

### 3.5 Configuration Interfaces

- **`Model<TApi>`** — id, name, api, provider, baseUrl, reasoning, thinkingLevelMap, input (["text"|"image"]), cost, contextWindow, maxTokens, headers, compat
- **`ImagesModel<TApi>`** — Like Model but with output (["text"|"image"]), no reasoning/contextWindow/maxTokens/compat
- **`StreamOptions`** — temperature, maxTokens, signal, apiKey, transport, cacheRetention, sessionId, onPayload, onResponse, headers, timeoutMs, maxRetries, maxRetryDelayMs, metadata
- **`SimpleStreamOptions`** — extends StreamOptions, adds `reasoning?: ThinkingLevel`, `thinkingBudgets?: ThinkingBudgets`
- **`ImagesOptions`** — signal, apiKey, onPayload, onResponse, headers, timeoutMs, maxRetries, maxRetryDelayMs, metadata
- **`ThinkingLevel`** — `"minimal" | "low" | "medium" | "high" | "xhigh"`
- **`ThinkingBudgets`** — `{ minimal?, low?, medium?, high? }`
- **`CacheRetention`** — `"none" | "short" | "long"`
- **`Transport`** — `"sse" | "websocket" | "websocket-cached" | "auto"`

### 3.6 Compatibility Interfaces

- **`OpenAICompletionsCompat`** — 20+ flags for OpenAI-compatible APIs (supportsStore, supportsDeveloperRole, supportsReasoningEffort, maxTokensField, requiresToolResultName, requiresAssistantAfterToolResult, requiresThinkingAsText, requiresReasoningContentOnAssistantMessages, thinkingFormat, openRouterRouting, vercelGatewayRouting, zaiToolStream, supportsStrictMode, cacheControlFormat, sendSessionAffinityHeaders, supportsLongCacheRetention)
- **`OpenAIResponsesCompat`** — sendSessionIdHeader, supportsLongCacheRetention
- **`AnthropicMessagesCompat`** — supportsEagerToolInputStreaming, supportsLongCacheRetention, sendSessionAffinityHeaders, supportsCacheControlOnTools
- **`OpenRouterRouting`** — allow_fallbacks, require_parameters, data_collection, zdr, enforce_distillable_text, order, only, ignore, quantizations, sort, max_price, preferred_min_throughput, preferred_max_latency
- **`VercelGatewayRouting`** — only, order

### 3.7 Stream Event Types

- **`AssistantMessageEvent`** — Union of:
  - `start`, `text_start`, `text_delta`, `text_end`
  - `thinking_start`, `thinking_delta`, `thinking_end`
  - `toolcall_start`, `toolcall_delta`, `toolcall_end`
  - `done` (with final message), `error` (with error message)

### 3.8 Function Signatures

- **`StreamFunction<TApi, TOptions>`** — `(model, context, options?) => AssistantMessageEventStream`
- **`ImagesFunction<TApi, TOptions>`** — `(model, context, options?) => Promise<AssistantImages>`

---

## 4. Core API Functions (from `src/stream.ts`)

| Function | Description |
|----------|-------------|
| `stream(model, context, options?)` | Stream a response as events |
| `complete(model, context, options?)` | Await complete response (returns Promise<AssistantMessage>) |
| `streamSimple(model, context, options?)` | Stream with thinking level support |
| `completeSimple(model, context, options?)` | Await complete response with thinking level |
| `getEnvApiKey(provider)` | Re-exported from env-api-keys.js; gets API key for a provider |

### Image Generation (from `src/images.ts`)

| Function | Description |
|----------|-------------|
| `generateImages(model, context, options?)` | Generate images (currently only OpenRouter) |

---

## 5. API Registry (from `src/api-registry.ts`)

| Export | Description |
|--------|-------------|
| `ApiProvider<TApi, TOptions>` | `{ api, stream, streamSimple }` |
| `ApiStreamFunction` | Typed alias for stream function |
| `ApiStreamSimpleFunction` | Typed alias for simple stream function |
| `registerApiProvider(provider, sourceId?)` | Register a provider implementation |
| `getApiProvider(api)` | Look up provider by api name |
| `getApiProviders()` | List all registered providers |
| `unregisterApiProviders(sourceId)` | Unregister providers from a given source |
| `clearApiProviders()` | Remove all providers |

### Images API Registry (from `src/images-api-registry.ts`)

| Export | Description |
|--------|-------------|
| `ImagesApiProvider<TApi, TOptions>` | `{ api, generateImages }` |
| `ImagesApiFunction` | Typed alias |
| `registerImagesApiProvider(provider, sourceId?)` | Register an image provider |
| `getImagesApiProvider(api)` | Look up image provider |

---

## 6. Model Registry (from `src/models.ts`)

| Function | Description |
|----------|-------------|
| `getModel(provider, modelId)` | Get a model by provider and model ID |
| `getProviders()` | List all known providers |
| `getModels(provider)` | Get all models for a provider |
| `calculateCost(model, usage)` | Compute monetary cost from token usage and model pricing |
| `getSupportedThinkingLevels(model)` | List thinking levels a model supports |
| `clampThinkingLevel(model, level)` | Clamp to nearest supported thinking level |
| `modelsAreEqual(a, b)` | Check two models are the same (id + provider) |

---

## 7. Image Model Registry (from `src/image-models.ts`)

| Function | Description |
|----------|-------------|
| `getImageModel(provider, modelId)` | Get an image model by provider and ID |
| `getImageProviders()` | List known image providers |
| `getImageModels(provider)` | Get all image models for a provider |

---

## 8. Environment API Keys (from `src/env-api-keys.ts`)

| Function | Description |
|----------|-------------|
| `findEnvKeys(provider)` | List configured env vars for a provider |
| `getEnvApiKey(provider)` | Get API key from process.env or /proc/self/environ |

### Supported Environment Variables by Provider

| Provider | Env Var |
|----------|---------|
| github-copilot | `COPILOT_GITHUB_TOKEN` |
| anthropic | `ANTHROPIC_OAUTH_TOKEN`, `ANTHROPIC_API_KEY` |
| openai | `OPENAI_API_KEY` |
| azure-openai-responses | `AZURE_OPENAI_API_KEY` (+ `AZURE_OPENAI_RESOURCE_NAME`, `AZURE_OPENAI_BASE_URL`, `AZURE_OPENAI_API_VERSION`, `AZURE_OPENAI_DEPLOYMENT_NAME_MAP`) |
| deepseek | `DEEPSEEK_API_KEY` |
| google | `GEMINI_API_KEY` |
| google-vertex | `GOOGLE_CLOUD_API_KEY` (or ADC via gcloud auth) |
| groq | `GROQ_API_KEY` |
| cerebras | `CEREBRAS_API_KEY` |
| xai | `XAI_API_KEY` |
| openrouter | `OPENROUTER_API_KEY` |
| vercel-ai-gateway | `AI_GATEWAY_API_KEY` |
| zai | `ZAI_API_KEY` |
| mistral | `MISTRAL_API_KEY` |
| minimax | `MINIMAX_API_KEY` |
| minimax-cn | `MINIMAX_CN_API_KEY` |
| moonshotai / moonshotai-cn | `MOONSHOT_API_KEY` |
| huggingface | `HF_TOKEN` |
| fireworks | `FIREWORKS_API_KEY` |
| together | `TOGETHER_API_KEY` |
| opencode / opencode-go | `OPENCODE_API_KEY` |
| kimi-coding | `KIMI_API_KEY` |
| cloudflare-workers-ai / cloudflare-ai-gateway | `CLOUDFLARE_API_KEY` (+ `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_GATEWAY_ID`) |
| xiaomi | `XIAOMI_API_KEY` |
| xiaomi-token-plan-cn | `XIAOMI_TOKEN_PLAN_CN_API_KEY` |
| xiaomi-token-plan-ams | `XIAOMI_TOKEN_PLAN_AMS_API_KEY` |
| xiaomi-token-plan-sgp | `XIAOMI_TOKEN_PLAN_SGP_API_KEY` |
| amazon-bedrock | `AWS_PROFILE`, `AWS_ACCESS_KEY_ID`+`AWS_SECRET_ACCESS_KEY`, `AWS_BEARER_TOKEN_BEDROCK`, ECS/IRSA |

---

## 9. Stream Event System (from `src/utils/event-stream.ts`)

| Class | Description |
|-------|-------------|
| `EventStream<T, R>` | Generic async-iterable stream with push/end/result pattern |
| `AssistantMessageEventStream` | Specialized EventStream for AssistantMessageEvent → AssistantMessage |
| `createAssistantMessageEventStream()` | Factory function for extensions |

### Methods

- `.push(event)` — Push an event
- `.end(result?)` — End the stream
- `[Symbol.asyncIterator]()` — Async iteration over events
- `.result()` — Promise that resolves to the final result (AssistantMessage)

---

## 10. Lazy Provider Module Registration (from `src/providers/register-builtins.ts`)

### Lazy-Loaded Stream/SimpleStream Functions

| Export | Loading |
|--------|---------|
| `streamAnthropic` / `streamSimpleAnthropic` | Lazy-loads `./anthropic.js` |
| `streamAzureOpenAIResponses` / `streamSimpleAzureOpenAIResponses` | Lazy-loads `./azure-openai-responses.js` |
| `streamGoogle` / `streamSimpleGoogle` | Lazy-loads `./google.js` |
| `streamGoogleVertex` / `streamSimpleGoogleVertex` | Lazy-loads `./google-vertex.js` |
| `streamMistral` / `streamSimpleMistral` | Lazy-loads `./mistral.js` |
| `streamOpenAICodexResponses` / `streamSimpleOpenAICodexResponses` | Lazy-loads `./openai-codex-responses.js` |
| `streamOpenAICompletions` / `streamSimpleOpenAICompletions` | Lazy-loads `./openai-completions.js` |
| `streamOpenAIResponses` / `streamSimpleOpenAIResponses` | Lazy-loads `./openai-responses.js` |
| `streamBedrockLazy` / `streamSimpleBedrockLazy` | Lazy-loads `./amazon-bedrock.js` (Node.js only) |

### Registration Functions

| Export | Description |
|--------|-------------|
| `registerBuiltInApiProviders()` | Register all 9 built-in providers |
| `resetApiProviders()` | Clear and re-register all built-in providers |
| `setBedrockProviderModule(module)` | Override the Bedrock provider module (for testing) |

### Auto-registration

`registerBuiltInApiProviders()` is called at module load time.

---

## 11. Provider Implementations

### 11.1 Anthropic (from `src/providers/anthropic.ts`)

| Export | Type |
|--------|------|
| `streamAnthropic` | `StreamFunction<"anthropic-messages", AnthropicOptions>` |
| `streamSimpleAnthropic` | `StreamFunction<"anthropic-messages", SimpleStreamOptions>` |
| `AnthropicEffort` | `"low" | "medium" | "high" | "xhigh" | "max"` |
| `AnthropicOptions` | thinkingEnabled, thinkingBudgetTokens, effort, thinkingDisplay, interleavedThinking, toolChoice, client |
| `AnthropicThinkingDisplay` | `"summarized" | "omitted"` |

**Features:** SSE streaming, PKCE OAuth, Claude Code identity headers, fine-grained/streaming beta, interleaved thinking, adaptive thinking (Opus 4.6+, Sonnet 4.6), budget-based thinking (older models), prompt caching, cache retention (short/long), tool call normalization, tool name canonicalization (Claude Code mode), redacted thinking handling.

### 11.2 OpenAI Completions (from `src/providers/openai-completions.ts`)

| Export | Type |
|--------|------|
| `streamOpenAICompletions` | `StreamFunction<"openai-completions", OpenAICompletionsOptions>` |
| `streamSimpleOpenAICompletions` | `StreamFunction<"openai-completions", SimpleStreamOptions>` |
| `OpenAICompletionsOptions` | toolChoice, reasoningEffort |
| `convertMessages` | Internal message converter (exported for extension use) |

**Features:** SSE streaming, thinking formats (OpenAI, OpenRouter, DeepSeek, Together, Z.AI, Qwen, Qwen Chat Template), reasoning_effort, Anthropic-style cache control (for OpenRouter Anthropic models), prompt_cache_key/retention, store, stream_options.include_usage, OpenRouter routing, Vercel AI Gateway routing, auto-detection of compat settings per provider/URL.

### 11.3 OpenAI Responses API (from `src/providers/openai-responses.ts`)

| Export | Type |
|--------|------|
| `streamOpenAIResponses` | `StreamFunction<"openai-responses", OpenAIResponsesOptions>` |
| `streamSimpleOpenAIResponses` | `StreamFunction<"openai-responses", SimpleStreamOptions>` |
| `OpenAIResponsesOptions` | reasoningEffort, reasoningSummary, serviceTier |

**Features:** SSE streaming, WebSocket support (via codex), reasoning.encrypted_content, service tier pricing (flex/priority), prompt_cache_key, session_id header.

### 11.4 OpenAI Codex Responses (from `src/providers/openai-codex-responses.ts`)

| Export | Type |
|--------|------|
| `streamOpenAICodexResponses` | `StreamFunction<"openai-codex-responses", OpenAICodexResponsesOptions>` |
| `streamSimpleOpenAICodexResponses` | `StreamFunction<"openai-codex-responses", SimpleStreamOptions>` |
| `OpenAICodexResponsesOptions` | reasoningEffort, reasoningSummary, serviceTier, textVerbosity |
| `OpenAICodexWebSocketDebugStats` | Stats: requests, connectionsCreated/Reused, cachedContextRequests, etc. |
| `getOpenAICodexWebSocketDebugStats(sessionId)` | Get WebSocket debug stats |
| `resetOpenAICodexWebSocketDebugStats(sessionId?)` | Reset debug stats |
| `closeOpenAICodexWebSocketSessions(sessionId?)` | Close WebSocket sessions |

**Features:** Dual transport (SSE + WebSocket), WebSocket session caching with continuation, automatic SSE fallback, retry logic with rate-limit handling, account JWT extraction, ChatGPT backend integration.

### 11.5 Azure OpenAI Responses (from `src/providers/azure-openai-responses.ts`)

| Export | Type |
|--------|------|
| `streamAzureOpenAIResponses` | `StreamFunction<"azure-openai-responses", AzureOpenAIResponsesOptions>` |
| `streamSimpleAzureOpenAIResponses` | `StreamFunction<"azure-openai-responses", SimpleStreamOptions>` |
| `AzureOpenAIResponsesOptions` | reasoningEffort, reasoningSummary, azureApiVersion, azureResourceName, azureBaseUrl, azureDeploymentName |

**Features:** Azure deployment name resolution via env map, multi-API-version support, Azure SDK integration.

### 11.6 Google Generative AI (from `src/providers/google.ts`)

| Export | Type |
|--------|------|
| `streamGoogle` | `StreamFunction<"google-generative-ai", GoogleOptions>` |
| `streamSimpleGoogle` | `StreamFunction<"google-generative-ai", SimpleStreamOptions>` |
| `GoogleOptions` | toolChoice, thinking (enabled, budgetTokens, level) |

**Features:** @google/genai SDK, thinking with levels (Gemini 3, Gemma 4) or budgets (Gemini 2.5), thought signatures, thinking signature retention, tool call ID deduplication, streaming content blocks.

### 11.7 Google Vertex AI (from `src/providers/google-vertex.ts`)

| Export | Type |
|--------|------|
| `streamGoogleVertex` | `StreamFunction<"google-vertex", GoogleVertexOptions>` |
| `streamSimpleGoogleVertex` | `StreamFunction<"google-vertex", SimpleStreamOptions>` |
| `GoogleVertexOptions` | toolChoice, thinking, project, location |

**Features:** Same as Google but with Vertex-specific auth (ADC or API key), project/location resolution, custom base URL support.

### 11.8 Google Shared (from `src/providers/google-shared.ts`)

| Export | Type |
|--------|------|
| `GoogleThinkingLevel` | `"THINKING_LEVEL_UNSPECIFIED" | "MINIMAL" | "LOW" | "MEDIUM" | "HIGH"` |
| `isThinkingPart(part)` | Detect thinking part |
| `retainThoughtSignature(existing, incoming)` | Preserve thought signatures during streaming |
| `convertMessages(model, context)` | Convert context to Gemini Content[] |
| `convertTools(tools, useParameters?)` | Convert tools to Gemini function declarations |
| `mapToolChoice(choice)` | Map to FunctionCallingConfigMode |
| `mapStopReason(reason)` | Map FinishReason to StopReason |
| `mapStopReasonString(reason)` | Map string finish reason to StopReason |
| `requiresToolCallId(modelId)` | Check if model needs explicit tool call IDs |

### 11.9 Mistral (from `src/providers/mistral.ts`)

| Export | Type |
|--------|------|
| `streamMistral` | `StreamFunction<"mistral-conversations", MistralOptions>` |
| `streamSimpleMistral` | `StreamFunction<"mistral-conversations", SimpleStreamOptions>` |
| `MistralOptions` | toolChoice, promptMode, reasoningEffort |

**Features:** Mistral SDK, thinking content streaming, tool call ID normalization (9-char alphanumeric), x-affinity headers for KV-cache, error formatting with status codes.

### 11.10 Amazon Bedrock (from `src/providers/amazon-bedrock.ts`)

| Export | Type |
|--------|------|
| `streamBedrock` | `StreamFunction<"bedrock-converse-stream", BedrockOptions>` |
| `streamSimpleBedrock` | `StreamFunction<"bedrock-converse-stream", SimpleStreamOptions>` |
| `BedrockOptions` | region, profile, toolChoice, reasoning, thinkingBudgets, interleavedThinking, thinkingDisplay, requestMetadata, bearerToken |
| `BedrockThinkingDisplay` | `"summarized" | "omitted"` |

**Features:** AWS SDK v3 ConverseStream API, SigV4 + bearer token auth, prompt caching with cache points, adaptive thinking (Claude 4.x), budget-based thinking (older Claude), HTTP proxy support (http_proxy, https_proxy, no_proxy), explicit endpoint resolution, GovCloud support, thinking signatures, region resolution.

### 11.11 Faux (Testing Provider) (from `src/providers/faux.ts`)

| Export | Type |
|--------|------|
| `registerFauxProvider(options?)` | Create a mock provider for testing |
| `fauxText(text)` | Create text content block |
| `fauxThinking(thinking)` | Create thinking content block |
| `fauxToolCall(name, args, options?)` | Create tool call block |
| `fauxAssistantMessage(content, options?)` | Create assistant message |

**Interfaces:**

```typescript
interface FauxModelDefinition { id, name?, reasoning?, input?, cost?, contextWindow?, maxTokens? }
interface RegisterFauxProviderOptions { api?, provider?, models?, tokensPerSecond?, tokenSize? }
interface FauxProviderRegistration {
  api, models, getModel(), state, setResponses(), appendResponses(),
  getPendingResponseCount(), unregister()
}
type FauxResponseStep = AssistantMessage | FauxResponseFactory;
type FauxResponseFactory = (context, options, state, model) => AssistantMessage | Promise<AssistantMessage>;
```

**Features:** Simulated streaming with configurable token speed, mock responses, prompt caching simulation, abort support.

---

## 12. Shared / Utility Modules

### 12.1 Simple Options (from `src/providers/simple-options.ts`)

| Export | Description |
|--------|-------------|
| `buildBaseOptions(model, options?, apiKey?)` | Extract common StreamOptions from SimpleStreamOptions |
| `clampReasoning(effort)` | Clamp "xhigh" to "high" |
| `adjustMaxTokensForThinking(baseMaxTokens, modelMaxTokens, reasoningLevel, customBudgets?)` | Compute maxTokens and thinkingBudget |

### 12.2 Transform Messages (from `src/providers/transform-messages.ts`)

| Export | Description |
|--------|-------------|
| `transformMessages(messages, model, normalizeToolCallId?)` | Cross-provider message transformation |

**Transformations performed:**
- Downgrades unsupported images to text placeholders
- Normalizes tool call IDs (OpenAI pipe-separated IDs, length limits)
- Converts thinking blocks to plain text for cross-model replay
- Drops redacted thinking for different models
- Preserves thinking signatures for same-model replay
- Inserts synthetic empty tool results for orphaned tool calls
- Strips thought signatures from cross-model tool calls
- Filters out errored/aborted assistant messages

### 12.3 GitHub Copilot Headers (from `src/providers/github-copilot-headers.ts`)

| Export | Description |
|--------|-------------|
| `inferCopilotInitiator(messages)` | Returns "user" or "agent" based on last message role |
| `hasCopilotVisionInput(messages)` | Check if messages contain images |
| `buildCopilotDynamicHeaders({ messages, hasImages })` | Build X-Initiator, Openai-Intent, Copilot-Vision-Request |

### 12.4 Cloudflare (from `src/providers/cloudflare.ts`)

| Export | Description |
|--------|-------------|
| `CLOUDFLARE_WORKERS_AI_BASE_URL` | Workers AI direct endpoint template |
| `CLOUDFLARE_AI_GATEWAY_COMPAT_BASE_URL` | AI Gateway Unified API template |
| `CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL` | AI Gateway -> OpenAI passthrough |
| `CLOUDFLARE_AI_GATEWAY_ANTHROPIC_BASE_URL` | AI Gateway -> Anthropic passthrough |
| `isCloudflareProvider(provider)` | Check if provider is Cloudflare |
| `resolveCloudflareBaseUrl(model)` | Substitute `{VAR}` placeholders from process.env |

### 12.5 OpenAI Prompt Cache (from `src/providers/openai-prompt-cache.ts`)

| Export | Description |
|--------|-------------|
| `CLAMP_OPENAI_PROMPT_CACHE_KEY_MAX_LENGTH` | 64 |
| `clampOpenAIPromptCacheKey(key)` | Truncate to 64 chars |

### 12.6 OpenAI Responses Shared (from `src/providers/openai-responses-shared.ts`)

| Export | Description |
|--------|-------------|
| `convertResponsesMessages(model, context, allowedToolCallProviders, options?)` | Convert to OpenAI Responses API format |
| `convertResponsesTools(tools, options?)` | Convert tools to OpenAI format |
| `processResponsesStream(openaiStream, output, stream, model, options?)` | Process Responses API stream events |
| `OpenAIResponsesStreamOptions` | serviceTier options |

### 12.7 Bedrock Provider Module (from `src/bedrock-provider.ts`)

| Export | Description |
|--------|-------------|
| `bedrockProviderModule` | `{ streamBedrock, streamSimpleBedrock }` — for runtime injection |

---

## 13. Utility Functions

### 13.1 JSON Utilities (from `src/utils/json-parse.ts`)

| Export | Description |
|--------|-------------|
| `repairJson(json)` | Repair malformed JSON strings (escape control chars, fix backslashes) |
| `parseJsonWithRepair<T>(json)` | Parse JSON with automatic repair on failure |
| `parseStreamingJson<T>(partialJson)` | Parse incomplete JSON from streaming (uses partial-json library) |

### 13.2 Diagnostics (from `src/utils/diagnostics.ts`)

| Export | Description |
|--------|-------------|
| `AssistantMessageDiagnostic` | `{ type, timestamp, error?, details? }` |
| `DiagnosticErrorInfo` | `{ name?, message, stack?, code? }` |
| `formatThrownValue(value)` | Convert unknown to string |
| `extractDiagnosticError(error)` | Extract DiagnosticErrorInfo from error |
| `createAssistantMessageDiagnostic(type, error, details?)` | Create diagnostic entry |
| `appendAssistantMessageDiagnostic(message, diagnostic)` | Append diagnostic to message |

### 13.3 Context Overflow Detection (from `src/utils/overflow.ts`)

| Export | Description |
|--------|-------------|
| `isContextOverflow(message, contextWindow?)` | Detect context overflow errors |
| `getOverflowPatterns()` | Get regex patterns (for testing) |

**Detection methods:**
1. Error message pattern matching (Anthropic, OpenAI, Google, xAI, Groq, Cerebras, Mistral, OpenRouter, Together, llama.cpp, LM Studio, GitHub Copilot, MiniMax, Kimi, Ollama)
2. Silent overflow: usage.input > contextWindow (z.ai style)
3. Length-stop overflow: output=0 + input≈contextWindow (Xiaomi MiMo style)

### 13.4 TypeBox Helpers (from `src/utils/typebox-helpers.ts`)

| Export | Description |
|--------|-------------|
| `StringEnum(values, options?)` | Create a string enum schema (for Google/other providers) |

### 13.5 Validation (from `src/utils/validation.ts`)

| Export | Description |
|--------|-------------|
| `validateToolCall(tools, toolCall)` | Find tool and validate arguments |
| `validateToolArguments(tool, toolCall)` | Validate and coerce tool call arguments against TypeBox schema |

**Features:** TypeBox schema validation, type coercion (string→number, etc.), union type handling, detailed error messages with paths.

### 13.6 Other Utilities

| File | Export | Description |
|------|--------|-------------|
| `utils/headers.ts` | `headersToRecord(headers)` | Convert Headers to Record<string, string> |
| `utils/hash.ts` | `shortHash(str)` | Deterministic 10-char hash (for tool call IDs) |
| `utils/sanitize-unicode.ts` | `sanitizeSurrogates(text)` | Remove unpaired Unicode surrogates |
| `utils/node-http-proxy.ts` | `createHttpProxyAgentsForTarget(targetUrl)` | Create HTTP proxy agents for Bedrock |
| `utils/node-http-proxy.ts` | `resolveHttpProxyUrlForTarget(targetUrl)` | Resolve proxy URL from env |
| `utils/node-http-proxy.ts` | `UNSUPPORTED_PROXY_PROTOCOL_MESSAGE` | Error message constant |

---

## 14. Session Resources (from `src/session-resources.ts`)

| Export | Description |
|--------|-------------|
| `registerSessionResourceCleanup(cleanup)` | Register a cleanup callback (returns deregister function) |
| `cleanupSessionResources(sessionId?)` | Run all registered cleanup callbacks |

Used by: OpenAI Codex WebSocket session management.

---

## 15. OAuth System (from `src/utils/oauth/index.ts`)

### Types (from `src/utils/oauth/types.ts`)

| Type | Description |
|------|-------------|
| `OAuthCredentials` | `{ refresh, access, expires, [key: string]: unknown }` |
| `OAuthProviderId` | `string` |
| `OAuthProvider` | **Deprecated** alias for `OAuthProviderId` |
| `OAuthPrompt` | `{ message, placeholder?, allowEmpty? }` |
| `OAuthAuthInfo` | `{ url, instructions? }` |
| `OAuthSelectOption` | `{ id, label }` |
| `OAuthSelectPrompt` | `{ message, options }` |
| `OAuthLoginCallbacks` | `{ onAuth, onPrompt, onProgress?, onManualCodeInput?, onSelect?, signal? }` |
| `OAuthProviderInterface` | `{ id, name, login, refreshToken, getApiKey, usesCallbackServer?, modifyModels? }` |

### Registry Functions

| Export | Description |
|--------|-------------|
| `getOAuthProvider(id)` | Look up OAuth provider by ID |
| `getOAuthProviders()` | List all registered OAuth providers |
| `getOAuthProviderInfoList()` | **Deprecated** — list with `{ id, name, available }` |
| `registerOAuthProvider(provider)` | Register a custom OAuth provider |
| `unregisterOAuthProvider(id)` | Unregister (restores built-in if applicable) |
| `resetOAuthProviders()` | Reset to built-in providers only |

### High-level API

| Export | Description |
|--------|-------------|
| `getOAuthApiKey(providerId, credentials)` | Get API key from OAuth credentials (auto-refreshes) |
| `refreshOAuthToken(providerId, credentials)` | **Deprecated** — refresh OAuth token |

### Built-in OAuth Providers

#### 15.1 Anthropic OAuth

| Export | Description |
|--------|-------------|
| `anthropicOAuthProvider` | Provider: `"anthropic"`, Name: "Anthropic (Claude Pro/Max)" |
| `loginAnthropic(options)` | PKCE authorization code flow with local callback server |
| `refreshAnthropicToken(refreshToken)` | Refresh with refresh_token grant |

**Flow:** Opens browser → local callback server on port 53692 → exchanges code → returns { access, refresh, expires }

#### 15.2 GitHub Copilot OAuth

| Export | Description |
|--------|-------------|
| `githubCopilotOAuthProvider` | Provider: `"github-copilot"`, Name: "GitHub Copilot" |
| `loginGitHubCopilot(options)` | Device code flow with optional enterprise domain |
| `refreshGitHubCopilotToken(refreshToken, enterpriseDomain?)` | Refresh via Copilot internal token endpoint |
| `getGitHubCopilotBaseUrl(token?, enterpriseDomain?)` | Resolve API base URL from proxy-ep |
| `normalizeDomain(input)` | Normalize a domain string |

**Flow:** Device code → poll for access token → fetch copilot token → enable all models

#### 15.3 OpenAI Codex OAuth

| Export | Description |
|--------|-------------|
| `openaiCodexOAuthProvider` | Provider: `"openai-codex"`, Name: "ChatGPT Plus/Pro (Codex Subscription)" |
| `loginOpenAICodex(options)` | PKCE flow with local callback server (port 1455) |
| `refreshOpenAICodexToken(refreshToken)` | Token refresh |

**Flow:** Browser flow or manual code paste → exchanges code → returns { access, refresh, expires, accountId }

---

## 16. CLI (from `src/cli.ts`)

**Binary:** `pi-ai` (via `npx @earendil-works/pi-ai`)

| Command | Description |
|---------|-------------|
| `login [provider]` | Login to an OAuth provider (interactive or direct) |
| `list` | List available OAuth providers |
| `help` / `--help` | Show help |

**Auth storage:** Saves credentials to `auth.json` in current directory.

---

## 17. Complete Provider List (from `src/models.generated.ts` & `src/image-models.generated.ts`)

### Chat/LLM Providers

| Provider Key | API | # Models | Notable Models |
|-------------|-----|----------|---------------|
| `amazon-bedrock` | `bedrock-converse-stream` | ~70+ | Nova, Claude (Haiku/Sonnet/Opus 4.x/4.7), DeepSeek, Llama, Mistral, Qwen, Gemini, GLM, Nemotron, MiniMax, Kimi, Palmyra |
| `anthropic` | `anthropic-messages` | ~25 | Claude 3/3.5/4 models (Haiku, Sonnet, Opus), latest aliases |
| `azure-openai-responses` | `azure-openai-responses` | ~45 | GPT-4o, GPT-4.1, GPT-5/5.1/5.2/5.3/5.4/5.5 series, o1, o3, o4-mini |
| `cerebras` | `openai-completions` | 4 | GPT OSS, Llama 3.1, Qwen 3, GLM-4.7 |
| `cloudflare-ai-gateway` | `anthropic-messages` | ~20 | Claude models via CF Gateway |
| `cloudflare-ai-gateway` | `openai-responses` | ~15 | GPT-4o/5, o1/o3/o4 via CF Gateway |
| `cloudflare-ai-gateway` | `openai-completions` | 4 | Kimi K2.5/2.6, Nemotron, GLM-4.7-Flash via CF Gateway |
| `cloudflare-workers-ai` | `openai-completions` | 8 | Gemma 4, Llama 4, Kimi K2.5/2.6, Nemotron, GPT OSS, GLM-4.7-Flash |
| `deepseek` | `openai-completions` | 2 | DeepSeek V4 Flash, DeepSeek V4 Pro |
| `fireworks` | `anthropic-messages` | ~20 | DeepSeek V3/V4, GLM 4.5/4.7/5/5.1, GPT OSS, Kimi K2/K2.5/K2.6, MiniMax M2.x, Qwen 3.6 |
| `github-copilot` | `anthropic-messages` | 6 | Claude Haiku/Sonnet/Opus 4.5/4.6/4.7 |
| `github-copilot` | `openai-completions` | 5 | GPT-4.1, GPT-4o, Gemini 2.5 Pro, Gemini 3 Flash, Gemini 3.1 Pro, Grok Code Fast |
| `github-copilot` | `openai-responses` | 7 | GPT-5-mini, GPT-5.2, GPT-5.2-codex, GPT-5.3-codex, GPT-5.4, GPT-5.4-mini, GPT-5.5 |
| `google` | `google-generative-ai` | ~30 | Gemini 1.5/2.0/2.5/3/3.1 series, Gemma 3/4, Flash/Pro/Lite/Live |
| `google-vertex` | `google-vertex` | ~15 | Gemini 1.5/2.0/2.5/3/3.1 series via Vertex AI |
| `groq` | `openai-completions` | ~18 | DeepSeek R1, Gemma 2, Llama 3/3.1/3.3/4, Mistral Saba, Kimi K2, GPT OSS, Qwen QwQ/Qwen3 |
| `huggingface` | `openai-completions` | ~20 | MiniMax M2.1/2.5/2.7, Qwen3 various, DeepSeek R1/V3/V4, Kimi K2/K2.5/K2.6, GLM-4.7/5/5.1, MiMo V2 Flash |
| `kimi-coding` | `anthropic-messages` | 2 | Kimi For Coding, Kimi K2 Thinking |
| `minimax` | `anthropic-messages` | 1 | MiniMax-M2.7 |
| `mistral` | `mistral-conversations` | varies | Mistral Small/Medium/Large, Pixtral, Ministral |
| `moonshotai` | `openai-completions` | varies | Kimi K2/K2.5/K2.6 |
| `openai` | `openai-completions` | varies | GPT-4o, GPT-4.1, GPT-5 series, o1/o3/o4-mini |
| `openai` | `openai-responses` | varies | GPT-4o, GPT-4.1, GPT-5 series, o1/o3/o4-mini |
| `openrouter` | `openai-completions` | varies | Wide range via OpenRouter |
| `together` | `openai-completions` | varies | Various models via Together AI |
| `vercel-ai-gateway` | `openai-completions` | varies | Via Vercel's AI Gateway |
| `xai` | `openai-completions` | varies | Grok models |
| `xiaomi` | `openai-completions` | varies | Xiaomi MiMo models |
| `zai` | `openai-completions` | varies | GLM-4.7, GLM-5 |

### Image Generation Providers

| Provider | API | # Models | Notable Models |
|----------|-----|----------|---------------|
| `openrouter` | `openrouter-images` | ~28 | FLUX.2 (Flex/Klein/Max/Pro), Seedream 4.5, Gemini 2.5 Flash Image, Gemini 3 Pro Image, Gemini 3.1 Flash Image, GPT-5 Image, GPT-5 Image Mini, GPT-5.4 Image 2, Recraft V3/V4/V4.1 series, Riverflow V2 series, OpenRouter Auto |

---

## 18. Package Subpath Exports

| Subpath | Import Path |
|---------|------------|
| `.` | Full library entry point |
| `./anthropic` | Anthropic provider module |
| `./azure-openai-responses` | Azure OpenAI Responses provider |
| `./google` | Google Generative AI provider |
| `./google-vertex` | Google Vertex AI provider |
| `./mistral` | Mistral provider |
| `./openai-codex-responses` | OpenAI Codex Responses provider |
| `./openai-completions` | OpenAI Completions provider |
| `./openai-responses` | OpenAI Responses provider |
| `./oauth` | OAuth module |
| `./bedrock-provider` | Bedrock provider module (for injection) |

---

## 19. Key Dependencies

| Dependency | Version | Purpose |
|-----------|---------|---------|
| `@anthropic-ai/sdk` | ^0.91.1 | Anthropic API client |
| `@aws-sdk/client-bedrock-runtime` | ^3.1030.0 | AWS Bedrock ConverseStream API |
| `@google/genai` | ^1.40.0 | Google Gemini API client |
| `@mistralai/mistralai` | ^2.2.0 | Mistral API client |
| `openai` | 6.26.0 | OpenAI/OpenRouter/Azure SDK |
| `partial-json` | ^0.1.7 | Streaming JSON partial parsing |
| `typebox` | ^1.1.24 | JSON Schema type system |
| `http-proxy-agent` | ^7.0.2 | HTTP proxy support |
| `https-proxy-agent` | ^7.0.6 | HTTPS proxy support |

---

## 20. Scripts

| Script | Description |
|--------|-------------|
| `generate-models` | Generate `models.generated.ts` from model definitions |
| `generate-image-models` | Generate `image-models.generated.ts` |
| `build` | Generate models + TypeScript build |
| `test` | Run Vitest test suite |
| `clean` | Remove dist directory |

---

## 21. Environment Variables (Cross-cutting)

| Variable | Purpose |
|----------|---------|
| `PI_CACHE_RETENTION` | Override default cache retention to "long" |
| `PI_OAUTH_CALLBACK_HOST` | OAuth callback server host (default: 127.0.0.1) |
| `AWS_BEDROCK_SKIP_AUTH` | Skip AWS auth for Bedrock (proxy mode) |
| `AWS_BEDROCK_FORCE_CACHE` | Force cache points on Bedrock (auto-detection override) |
| `AWS_BEDROCK_FORCE_HTTP1` | Force HTTP/1.1 for Bedrock |
| `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` / `ALL_PROXY` | HTTP proxy support (Bedrock, OpenAI Codex WebSocket) |
