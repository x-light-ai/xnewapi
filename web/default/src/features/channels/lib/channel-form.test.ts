/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Protect third-party Codex protocol persistence in the default form.

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformFormDataToCreatePayload,
} from './channel-form'

describe('channel upstream protocol', () => {
  test('persists Codex only for Claude-compatible channels', () => {
    const claudePayload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'claude-codex',
      type: 14,
      models: 'claude-sonnet-4',
      upstream_protocol: 'codex',
    })
    const claudeSettings = JSON.parse(String(claudePayload.channel.settings))
    assert.equal(claudeSettings.upstream_protocol, 'codex')

    const openAIPayload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'openai',
      type: 1,
      models: 'gpt-4.1',
      settings: JSON.stringify({ upstream_protocol: 'codex' }),
      upstream_protocol: 'codex',
    })
    const openAISettings = JSON.parse(String(openAIPayload.channel.settings))
    assert.equal('upstream_protocol' in openAISettings, false)
  })
})
