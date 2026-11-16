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

import { useEffect, useState } from 'react';
import { API } from '../../helpers';

const CACHE_MS = 60 * 1000;

let cached = null;
let inflight = null;

function loadSuccessRates() {
  if (cached && Date.now() - cached.at < CACHE_MS) {
    return Promise.resolve(cached.rates);
  }
  if (!inflight) {
    inflight = API.get('/api/perf-metrics/success-windows', {
      skipErrorHandler: true,
    })
      .then((res) => {
        const models = res.data?.success ? res.data?.data?.models || [] : [];
        const rates = {};
        models.forEach((model) => {
          if (model?.model_name) {
            rates[model.model_name] = model;
          }
        });
        cached = { at: Date.now(), rates };
        return rates;
      })
      .finally(() => {
        inflight = null;
      });
  }
  return inflight;
}

export function useModelSuccessRates() {
  const fresh = cached && Date.now() - cached.at < CACHE_MS;
  const [rates, setRates] = useState(() => (fresh ? cached.rates : {}));
  const [loading, setLoading] = useState(() => !fresh);

  useEffect(() => {
    let active = true;
    loadSuccessRates()
      .then((next) => {
        if (!active) return;
        setRates(next || {});
        setLoading(false);
      })
      .catch(() => {
        if (!active) return;
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  return { rates, loading };
}
