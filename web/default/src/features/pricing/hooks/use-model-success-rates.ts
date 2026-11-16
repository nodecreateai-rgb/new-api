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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPerfSuccessWindows } from '@/features/performance-metrics/api'
import type { ModelSuccessRateWindows } from '../lib/success-rate'

function finiteRate(value: number | null | undefined): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return null
  }
  return value
}

export function useModelSuccessRateMap() {
  const query = useQuery({
    queryKey: ['perf-metrics-success-windows'],
    queryFn: getPerfSuccessWindows,
    staleTime: 60 * 1000,
    retry: false,
  })

  const rates = useMemo(() => {
    const map = new Map<string, ModelSuccessRateWindows>()
    for (const model of query.data?.data?.models ?? []) {
      map.set(model.model_name, {
        rate24h: finiteRate(model.success_rate_24h),
        rate12h: finiteRate(model.success_rate_12h),
        rate3h: finiteRate(model.success_rate_3h),
        avgGenerationSec: finiteRate(model.avg_generation_sec),
      })
    }
    return map
  }, [query.data])

  return {
    rates,
    isLoading: query.isPending,
  }
}
