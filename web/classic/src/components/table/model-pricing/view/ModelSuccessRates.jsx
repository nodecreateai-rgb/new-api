/*
Copyright (C) 2025 QuantumNous

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

import React from 'react';

const WINDOWS = [
  { hours: 24, field: 'success_rate_24h' },
  { hours: 12, field: 'success_rate_12h' },
  { hours: 3, field: 'success_rate_3h' },
];

function displayedSuccessPercent(rate) {
  return Math.round(rate * 10) / 10;
}

function formatSuccessPercent(rate) {
  const shown = displayedSuccessPercent(rate);
  if (Number.isInteger(shown)) {
    return String(shown);
  }
  return shown.toFixed(1);
}

function successRateClass(rate, loading) {
  if (loading || rate == null) {
    return 'bg-gray-100 text-gray-500 dark:bg-white/10 dark:text-gray-300';
  }
  const shown = displayedSuccessPercent(rate);
  if (shown > 90) {
    return 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300';
  }
  if (shown >= 65) {
    return 'bg-orange-500/15 text-orange-700 dark:text-orange-300';
  }
  return 'bg-red-500/15 text-red-700 dark:text-red-300';
}

const ModelSuccessRates = ({ rates, loading = false, t }) => {
  return (
    <div className='flex flex-wrap items-center gap-1'>
      {WINDOWS.map((period) => {
        const raw = loading ? null : rates?.[period.field];
        const rate =
          typeof raw === 'number' && Number.isFinite(raw) ? raw : null;
        const value = rate == null ? '—' : `${formatSuccessPercent(rate)}%`;
        let description = t('加载中...');
        if (!loading && rate == null) {
          description = t('最近 {{hours}} 小时没有请求', {
            hours: period.hours,
          });
        } else if (!loading && rate != null) {
          description = t('{{hours}}小时成功率：{{rate}}%', {
            hours: period.hours,
            rate: formatSuccessPercent(rate),
          });
        }

        return (
          <span
            key={period.hours}
            title={description}
            aria-label={description}
            className={`inline-flex items-baseline gap-1 whitespace-nowrap rounded-md px-1.5 py-0.5 text-xs font-semibold tabular-nums ${
              loading ? 'animate-pulse' : ''
            } ${successRateClass(rate, loading)}`}
          >
            <span className='font-medium'>{period.hours}h</span>
            <span>{value}</span>
          </span>
        );
      })}
    </div>
  );
};

export default ModelSuccessRates;
