/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type {
  ChatCompletionRequest,
  VideoGenerationRequest,
  Message,
  PlaygroundConfig,
  ParameterEnabled,
} from '../types'
import { formatMessageForAPI, isValidMessage } from './message-utils'

/**
 * Build API request payload from messages and config
 */
export function buildChatCompletionPayload(
  messages: Message[],
  config: PlaygroundConfig,
  parameterEnabled: ParameterEnabled
): ChatCompletionRequest {
  // Filter and format valid messages
  const processedMessages = messages
    .filter(isValidMessage)
    .map(formatMessageForAPI)

  const payload: ChatCompletionRequest = {
    model: config.model,
    group: config.group,
    messages: processedMessages,
    stream: config.stream,
  }

  // Add enabled parameters
  const parameterKeys: Array<keyof ParameterEnabled> = [
    'temperature',
    'top_p',
    'max_tokens',
    'frequency_penalty',
    'presence_penalty',
    'seed',
  ]

  parameterKeys.forEach((key) => {
    if (parameterEnabled[key]) {
      const value = config[key as keyof PlaygroundConfig]
      if (value !== undefined && value !== null) {
        ;(payload as unknown as Record<string, unknown>)[key] = value
      }
    }
  })

  return payload
}


export function isPlaygroundVideoModel(model: string): boolean {
  const normalized = model.trim().toLowerCase()
  if (!normalized) return false
  if (/^sd2-c\d+$/.test(normalized)) return true
  if (/^seedance/.test(normalized)) return true
  if (/^doubao-seedance/.test(normalized)) return true
  if (/^sora/.test(normalized)) return true
  if (/^veo/.test(normalized)) return true
  if (/^kling/.test(normalized)) return true
  if (/^hailuo/.test(normalized)) return true
  return false
}

export function buildVideoGenerationPayload(
  messages: Message[],
  config: PlaygroundConfig
): VideoGenerationRequest {
  const lastUserMessage = [...messages]
    .reverse()
    .find((message) => message.from === 'user')
  const prompt = lastUserMessage?.versions[0]?.content?.trim() || ''

  return {
    model: config.model,
    group: config.group,
    prompt,
    seconds: '5',
    size: '1280x720',
  }
}
