-- sd2-c6 per-call price aligned with sd2-c7 (¥0.5); vip2 keeps c6/c7 at ¥0.3
UPDATE options
SET value = (jsonb_set(value::jsonb, '{sd2-c6}', '0.5'))::text
WHERE key = 'ModelPrice'
  AND (value::jsonb->>'sd2-c6') IS DISTINCT FROM '0.5';

UPDATE options
SET value = (
  SELECT jsonb_object_agg(
    grp.key,
    CASE
      WHEN grp.key = 'vip2' THEN jsonb_set(grp.value, '{sd2-c6}', '0.3')
      ELSE jsonb_set(grp.value, '{sd2-c6}', '0.5')
    END
  )::text
  FROM jsonb_each(value::jsonb) AS grp(key, value)
)
WHERE key = 'ModelGroupPrice'
  AND EXISTS (
    SELECT 1 FROM jsonb_each(value::jsonb) AS grp(key, val)
    WHERE (grp.key = 'vip2' AND val->>'sd2-c6' IS DISTINCT FROM '0.3')
       OR (grp.key <> 'vip2' AND val ? 'sd2-c6' AND val->>'sd2-c6' IS DISTINCT FROM '0.5')
  );
