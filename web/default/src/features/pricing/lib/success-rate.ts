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

export type ModelSuccessRateWindows = {
  rate24h: number | null
  rate12h: number | null
  rate3h: number | null
  avgGenerationSec: number | null
}

export type SuccessRateLevel = 'good' | 'warn' | 'bad'

export const SUCCESS_RATE_WINDOWS = [
  { hours: 24, key: 'rate24h' },
  { hours: 12, key: 'rate12h' },
  { hours: 3, key: 'rate3h' },
] as const

export function displayedSuccessPercent(rate: number): number {
  return Math.round(rate * 10) / 10
}

export function formatSuccessPercent(rate: number): string {
  const shown = displayedSuccessPercent(rate)
  if (Number.isInteger(shown)) {
    return String(shown)
  }
  return shown.toFixed(1)
}

// Above 90% is green. 65% through 90% is orange. Below 65% is red.
// The level follows the rounded number shown on the badge.
export function successRateLevel(rate: number): SuccessRateLevel {
  const shown = displayedSuccessPercent(rate)
  if (shown > 90) {
    return 'good'
  }
  if (shown >= 65) {
    return 'warn'
  }
  return 'bad'
}
