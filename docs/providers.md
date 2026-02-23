# LLM Providers

## Overview

EazyClaw supports 6 LLM providers. Set API keys as environment variables — the first available provider is used by default.

## Provider Table

| Provider | Env Var | Default Model | Notes |
|---|---|---|---|
| Kimi Coding | `KIMI_API_KEY` | k2p5 | Recommended — cheapest Claude-quality option (~$9/mo) |
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4-6 | Direct Claude access |
| OpenAI | `OPENAI_API_KEY` | gpt-4.1 | GPT models |
| Google Gemini | `GEMINI_API_KEY` | gemini-2.5-flash | Large context window |
| Moonshot | `MOONSHOT_API_KEY` | kimi-k2.5 | Alternative Kimi access |
| Zhipu | `ZHIPU_API_KEY` | glm-4-plus | Chinese language models |

## Recommended: Kimi Coding

Kimi Coding is the cheapest way to get Claude-level intelligence:

- ~$9/month for the Coding plan
- Sign up at kimi.com, subscribe to the Coding plan, and copy your API key (it has the `sk-kimi-` prefix)
- Can swap to direct Anthropic or OpenAI at any time by setting the corresponding env var

## Default Model

The default model is set in `config.yaml` under `providers.default_model`. If not set, EazyClaw uses the first available provider based on which API keys are present in the environment.

```yaml
providers:
  default_model: k2p5
```

## Multi-Provider Setup

Set multiple API keys to enable fallback options. Each provider section in `config.yaml` can specify its own model. Routing uses `default_model` to select the active provider.

```yaml
providers:
  default_model: k2p5
  kimi:
    model: k2p5
  anthropic:
    model: claude-sonnet-4-6
  openai:
    model: gpt-4.1
  gemini:
    model: gemini-2.5-flash
```

With multiple keys set, EazyClaw falls back to the next available provider if the primary one is unavailable.
