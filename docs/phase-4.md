# Phase 4: The Voice

**Timeline: 1 week**
**Depends on: Phase 3**
**Goal: Bidirectional voice conversations**

## Tasks

### 1. Voice Server (Days 1-2)
- Deploy Piper TTS in kora-voice namespace
- STT: Whisper via existing PicoClaw capability or deploy whisper.cpp container
- Internal ClusterIP service

### 2. Telegram Voice Integration (Days 3-4)
- Receive Telegram voice messages → STT → text → agent loop → response
- Response text → Piper TTS → voice message back to user
- Streaming where possible

### 3. Voice Personality (Day 5)
- Tune Piper voice model to match Kora's personality
- Test conversation flow with voice

## Success Criteria
- [ ] Voice messages through Telegram work bidirectionally
- [ ] Latency under 3 seconds for voice response
