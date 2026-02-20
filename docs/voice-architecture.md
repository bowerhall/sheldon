# Voice Architecture

## Overview

Kora's voice interface uses a split architecture to optimize resources and latency.

## Components

### Speech-to-Text (STT) - Client-side
- **Model**: Whisper small (~500MB)
- **Runtime**: whisper.cpp bundled in Mac app
- **Why client-side**:
  - Privacy: user speech never leaves device
  - Latency: instant, no network round-trip
  - Resources: offloads heavy model from server

### Text-to-Speech (TTS) - Server-side
- **Model**: Piper (~500MB-1GB)
- **Runtime**: k8s pod with HTTP API
- **Why server-side**:
  - Lighter model than STT
  - Streamable audio masks latency
  - CX32 can handle it alongside Kora

## Mac App Behavior

```
App idle       → whisper process running, model unloaded (~50MB)
Widget opens   → load model (~1-2 sec first time)
Widget active  → model warm, instant transcription
Widget idle    → keep model warm
App background → unload model, back to ~50MB
```

## Resource Summary

| Component | Location | Memory | Latency |
|-----------|----------|--------|---------|
| Whisper STT | Mac (local) | ~500MB active, ~50MB idle | Instant |
| Piper TTS | k8s cluster | ~500MB-1GB | ~100-200ms + stream |

## Bundle Size

Mac app total: ~550MB
- whisper.cpp binary: ~50MB
- whisper-small model: ~500MB

## Future Options

- Offer model size toggle (base 150MB vs small 500MB)
- Cache frequently used TTS responses
- Add wake word detection (local, tiny model)
