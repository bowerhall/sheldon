# Voice System

## Architecture (Phase 4+)

```
User voice msg → Telegram → Kora
  → STT (Whisper) → text → agent loop → response text
  → TTS (Piper) → audio → Telegram → User
```

## Components

- **STT**: Whisper.cpp or PicoClaw built-in (depending on what's available)
- **TTS**: Piper (lightweight, runs on CPU) deployed in kora-voice namespace
- **Streaming**: Where possible, stream TTS output to reduce perceived latency

## Voice Personality

Piper voice model tuned to match Kora's personality. Warm, clear, natural cadence.

## Future (Phase 5+)

Mac app: direct microphone access, lower latency than Telegram voice messages.
Mobile (Phase 7): native audio handling, push-to-talk.
