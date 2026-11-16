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
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  SUCCESS_RATE_WINDOWS,
  formatSuccessPercent,
  successRateLevel,
  type ModelSuccessRateWindows,
} from '../lib/success-rate'

export interface ModelSuccessRatesProps extends React.HTMLAttributes<HTMLDivElement> {
  rates?: ModelSuccessRateWindows
  loading?: boolean
}

function rateStyle(
  rate: number | null,
  loading: boolean
): React.CSSProperties {
  if (loading || rate == null) {
    return { background: 'rgba(148, 163, 184, 0.2)', color: '#64748b' }
  }
  const level = successRateLevel(rate)
  if (level === 'good') {
    return { background: 'rgba(16, 185, 129, 0.18)', color: '#047857' }
  }
  if (level === 'warn') {
    return { background: 'rgba(249, 115, 22, 0.18)', color: '#c2410c' }
  }
  return { background: 'rgba(239, 68, 68, 0.18)', color: '#b91c1c' }
}

export const ModelSuccessRates = memo(function ModelSuccessRates(
  props: ModelSuccessRatesProps
) {
  const { t } = useTranslation()
  const loading = props.loading ?? false

  return (
    <div
      className={cn('flex flex-wrap items-center gap-1', props.className)}
      aria-label={t('Success rate')}
    >
      {SUCCESS_RATE_WINDOWS.map((period) => {
        const rate = loading ? null : (props.rates?.[period.key] ?? null)
        const value = rate == null ? '—' : `${formatSuccessPercent(rate)}%`
        let description = t('Loading...')
        if (!loading && rate == null) {
          description = t('No requests in the last {{hours}} hours', {
            hours: period.hours,
          })
        } else if (!loading && rate != null) {
          description = t('{{hours}}-hour success rate: {{rate}}%', {
            hours: period.hours,
            rate: formatSuccessPercent(rate),
          })
        }

        return (
          <span
            key={period.hours}
            title={description}
            aria-label={description}
            className={cn(
              'inline-flex items-baseline gap-1 rounded-md px-1.5 py-0.5 text-xs leading-4 font-semibold tabular-nums whitespace-nowrap',
              loading && 'animate-pulse'
            )}
            style={rateStyle(rate, loading)}
          >
            <span className='font-medium'>{period.hours}h</span>
            <span>{value}</span>
          </span>
        )
      })}
    </div>
  )
})
