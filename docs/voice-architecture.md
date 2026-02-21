# Voice Architecture

## Overview

Sheldon's voice interface uses a split architecture: STT runs locally on the Mac, TTS runs on the VPS. This optimizes for latency, privacy, and resource efficiency.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  MAC APP                                                            │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  User speaks                                                 │   │
│  │       │                                                      │   │
│  │       ▼                                                      │   │
│  │  ┌──────────────┐                                           │   │
│  │  │ whisper.cpp  │  STT (local, ~500MB model)                │   │
│  │  │ tiny/base    │  Only needs to understand YOUR voice      │   │
│  │  └──────────────┘                                           │   │
│  │       │                                                      │   │
│  │       ▼ (text)                                               │   │
│  └───────┼──────────────────────────────────────────────────────┘   │
│          │                                                          │
└──────────┼──────────────────────────────────────────────────────────┘
           │ WebSocket/HTTP
           ▼
┌──────────────────────────────────────────────────────────────────────┐
│  VPS (Hetzner CX32, 8GB)                                             │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  Sheldon                                                        │ │
│  │       │                                                         │ │
│  │       ▼ (response text)                                         │ │
│  │  ┌──────────────┐                                              │ │
│  │  │  Piper TTS   │  Configurable voices                         │ │
│  │  │  (~100MB)    │  User picks how Sheldon sounds               │ │
│  │  └──────────────┘                                              │ │
│  │       │                                                         │ │
│  │       ▼ (audio stream)                                          │ │
│  └───────┼─────────────────────────────────────────────────────────┘ │
│          │                                                           │
└──────────┼───────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────────────────┐
│  MAC APP                                                             │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  Audio playback (streams as it arrives)                        │ │
│  └────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
```

## Speech-to-Text (STT) - Mac App

**Location**: Bundled with Mac app (whisper.cpp)

**Model**: Whisper tiny or base
- tiny: ~75MB, fastest, good for single speaker
- base: ~150MB, better accuracy

**Why a small model is fine**:
- Only needs to understand ONE voice (yours)
- No multi-accent support needed
- Can be fine-tuned on your speech patterns over time
- Larger models offer multi-speaker/accent support you don't need

**Why client-side**:
- **Privacy**: Speech never leaves your device
- **Latency**: Instant transcription, no network round-trip
- **Offline**: Works without internet (text queues until connected)
- **Resources**: Offloads compute from VPS

**Implementation**: whisper.cpp binary bundled with app
- C++ binary, no Python runtime needed
- ~50MB binary + model file
- Fast startup, low memory when idle

## Text-to-Speech (TTS) - VPS

**Location**: Docker container on VPS

**Model**: Piper TTS
- Lightweight: ~50-100MB per voice
- Fast CPU inference (no GPU needed)
- High quality output
- Multiple voice options

**Why server-side**:
- Lighter than STT models
- Audio streaming masks network latency
- Centralized voice configuration
- VPS can easily handle it alongside Sheldon

**Configurable voices**:
- User selects Sheldon's voice in settings
- Options: different accents, genders, tones
- Piper has 100+ pretrained voices
- Can add custom voices later (voice cloning)

**Voice selection examples**:
```
voices/
├── en_US-lessac-medium.onnx    # American male
├── en_GB-alan-medium.onnx      # British male
├── en_US-libritts-high.onnx    # American female
└── en_AU-karen-medium.onnx     # Australian female
```

## Mac App Behavior

```
App launched    → whisper.cpp process starts, model NOT loaded (~10MB)
Voice activated → load model into memory (~500MB, 1-2 sec first time)
Listening       → model warm, instant transcription
Idle (30 sec)   → keep model warm (for quick re-activation)
App background  → unload model, back to ~10MB
App quit        → process exits
```

## Resource Summary

| Component | Location | Memory | Latency |
|-----------|----------|--------|---------|
| Whisper STT | Mac (local) | ~150MB active (base model) | Instant |
| Piper TTS | VPS (Docker) | ~100MB | ~50-100ms + stream |

**VPS total for voice**: ~100MB additional. 8GB CX32 is plenty.

## Communication Flow

1. User presses hotkey or clicks mic button
2. Mac app starts recording
3. User speaks, releases button (or silence detection)
4. whisper.cpp transcribes locally → text
5. Text sent to Sheldon via WebSocket
6. Sheldon processes, generates response
7. Response text sent to Piper TTS on VPS
8. Audio streamed back to Mac app
9. Mac app plays audio as it arrives

**Total latency**: ~200-500ms from speech end to audio start
- STT: instant (local)
- Network to VPS: ~20-50ms
- Sheldon thinking: ~100-300ms
- TTS generation: ~50-100ms
- Network back: ~20-50ms
- Audio starts streaming immediately

## Mac App Bundle Size

```
Sheldon.app/
├── Contents/
│   ├── MacOS/
│   │   ├── Sheldon          # Main app binary
│   │   └── whisper          # whisper.cpp binary (~5MB)
│   └── Resources/
│       └── models/
│           └── ggml-base.bin  # Whisper base model (~150MB)
```

**Total**: ~160MB (with base model)

Option to download larger model (small: 500MB) for better accuracy.

## Telegram Voice (Secondary)

Telegram remains available but is secondary to the Mac app:

- Voice messages in Telegram → transcribed server-side (Groq Whisper API)
- Sheldon responds with text (TTS optional via voice note)
- No local model involved, uses cloud API

This keeps VPS resources light - Groq API handles the heavy STT work for Telegram voice messages.

## Future Enhancements

1. **Wake word detection**: Tiny local model (~5MB) for "Hey Sheldon"
2. **Voice activity detection**: Auto-start/stop recording
3. **Fine-tuning**: Adapt Whisper to your voice for better accuracy
4. **Custom TTS voice**: Clone a specific voice for Sheldon
5. **Streaming STT**: Transcribe while speaking (real-time feedback)
