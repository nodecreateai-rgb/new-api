
CREATE TABLE IF NOT EXISTS `abilities` (
  `group` String,
  `model` String,
  `channel_id` Int64,
  `enabled` Nullable(UInt8),
  `priority` Nullable(Int64),
  `weight` Nullable(UInt64),
  `tag` Nullable(String)
) ENGINE=MergeTree ORDER BY (`group`, `model`, `channel_id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `channels` (
  `id` Int64,
  `type` Nullable(Int64),
  `key` String,
  `open_ai_organization` Nullable(String),
  `test_model` Nullable(String),
  `status` Nullable(Int64),
  `name` Nullable(String),
  `weight` Nullable(UInt64),
  `created_time` Nullable(Int64),
  `test_time` Nullable(Int64),
  `response_time` Nullable(Int64),
  `base_url` Nullable(String),
  `other` Nullable(String),
  `balance` Nullable(Float64),
  `balance_updated_time` Nullable(Int64),
  `models` Nullable(String),
  `group` Nullable(String),
  `used_quota` Nullable(Int64),
  `model_mapping` Nullable(String),
  `status_code_mapping` Nullable(String),
  `priority` Nullable(Int64),
  `auto_ban` Nullable(Int64),
  `other_info` Nullable(String),
  `tag` Nullable(String),
  `setting` Nullable(String),
  `param_override` Nullable(String),
  `header_override` Nullable(String),
  `remark` Nullable(String),
  `channel_info` Nullable(String),
  `settings` Nullable(String)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `checkins` (
  `id` Int64,
  `user_id` Int64,
  `checkin_date` String,
  `quota_awarded` Int64,
  `created_at` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `custom_oauth_providers` (
  `id` Int64,
  `name` String,
  `slug` String,
  `icon` Nullable(String),
  `enabled` Nullable(UInt8),
  `client_id` Nullable(String),
  `client_secret` Nullable(String),
  `authorization_endpoint` Nullable(String),
  `token_endpoint` Nullable(String),
  `user_info_endpoint` Nullable(String),
  `scopes` Nullable(String),
  `user_id_field` Nullable(String),
  `username_field` Nullable(String),
  `display_name_field` Nullable(String),
  `email_field` Nullable(String),
  `well_known` Nullable(String),
  `auth_style` Nullable(Int64),
  `access_policy` Nullable(String),
  `access_denied_message` Nullable(String),
  `created_at` Nullable(DateTime64(3)),
  `updated_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `logs` (
  `id` Int64,
  `user_id` Nullable(Int64),
  `created_at` Nullable(Int64),
  `type` Nullable(Int64),
  `content` Nullable(String),
  `username` Nullable(String),
  `token_name` Nullable(String),
  `model_name` Nullable(String),
  `quota` Nullable(Int64),
  `prompt_tokens` Nullable(Int64),
  `completion_tokens` Nullable(Int64),
  `use_time` Nullable(Int64),
  `is_stream` Nullable(UInt8),
  `channel_id` Nullable(Int64),
  `channel_name` Nullable(String),
  `token_id` Nullable(Int64),
  `group` Nullable(String),
  `ip` Nullable(String),
  `request_id` Nullable(String),
  `upstream_request_id` Nullable(String),
  `other` Nullable(String)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `midjourneys` (
  `id` Int64,
  `code` Nullable(Int64),
  `user_id` Nullable(Int64),
  `action` Nullable(String),
  `mj_id` Nullable(String),
  `prompt` Nullable(String),
  `prompt_en` Nullable(String),
  `description` Nullable(String),
  `state` Nullable(String),
  `submit_time` Nullable(Int64),
  `start_time` Nullable(Int64),
  `finish_time` Nullable(Int64),
  `image_url` Nullable(String),
  `video_url` Nullable(String),
  `video_urls` Nullable(String),
  `status` Nullable(String),
  `progress` Nullable(String),
  `fail_reason` Nullable(String),
  `channel_id` Nullable(Int64),
  `quota` Nullable(Int64),
  `buttons` Nullable(String),
  `properties` Nullable(String)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `models` (
  `id` Int64,
  `model_name` String,
  `description` Nullable(String),
  `icon` Nullable(String),
  `tags` Nullable(String),
  `vendor_id` Nullable(Int64),
  `endpoints` Nullable(String),
  `status` Nullable(Int64),
  `sync_official` Nullable(Int64),
  `created_time` Nullable(Int64),
  `updated_time` Nullable(Int64),
  `deleted_at` Nullable(DateTime64(3)),
  `name_rule` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `options` (
  `key` String,
  `value` Nullable(String)
) ENGINE=MergeTree ORDER BY (`key`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `passkey_credentials` (
  `id` Int64,
  `user_id` Int64,
  `credential_id` String,
  `public_key` String,
  `attestation_type` Nullable(String),
  `aa_guid` Nullable(String),
  `sign_count` Nullable(Int32),
  `clone_warning` Nullable(UInt8),
  `user_present` Nullable(UInt8),
  `user_verified` Nullable(UInt8),
  `backup_eligible` Nullable(UInt8),
  `backup_state` Nullable(UInt8),
  `transports` Nullable(String),
  `attachment` Nullable(String),
  `last_used_at` Nullable(DateTime64(3)),
  `created_at` Nullable(DateTime64(3)),
  `updated_at` Nullable(DateTime64(3)),
  `deleted_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `perf_metrics` (
  `id` Int64,
  `model_name` Nullable(String),
  `group` Nullable(String),
  `bucket_ts` Nullable(Int64),
  `request_count` Nullable(Int64),
  `success_count` Nullable(Int64),
  `total_latency_ms` Nullable(Int64),
  `ttft_sum_ms` Nullable(Int64),
  `ttft_count` Nullable(Int64),
  `output_tokens` Nullable(Int64),
  `generation_ms` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `prefill_groups` (
  `id` Int64,
  `name` String,
  `type` String,
  `items` Nullable(String),
  `description` Nullable(String),
  `created_time` Nullable(Int64),
  `updated_time` Nullable(Int64),
  `deleted_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `quota_data` (
  `id` Int64,
  `user_id` Nullable(Int64),
  `username` Nullable(String),
  `model_name` Nullable(String),
  `created_at` Nullable(Int64),
  `token_used` Nullable(Int64),
  `count` Nullable(Int64),
  `quota` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `redemptions` (
  `id` Int64,
  `user_id` Nullable(Int64),
  `key` Nullable(String),
  `status` Nullable(Int64),
  `name` Nullable(String),
  `quota` Nullable(Int64),
  `created_time` Nullable(Int64),
  `redeemed_time` Nullable(Int64),
  `used_user_id` Nullable(Int64),
  `deleted_at` Nullable(DateTime64(3)),
  `expired_time` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `setups` (
  `id` UInt64,
  `version` String,
  `initialized_at` Int64
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `subscription_orders` (
  `id` Int64,
  `user_id` Nullable(Int64),
  `plan_id` Nullable(Int64),
  `money` Nullable(Float64),
  `trade_no` Nullable(String),
  `payment_method` Nullable(String),
  `payment_provider` Nullable(String),
  `status` Nullable(String),
  `create_time` Nullable(Int64),
  `complete_time` Nullable(Int64),
  `provider_payload` Nullable(String)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `subscription_plans` (
  `id` Int64,
  `title` String,
  `subtitle` Nullable(String),
  `price_amount` Float64,
  `currency` String,
  `duration_unit` String,
  `duration_value` Int64,
  `custom_seconds` Int64,
  `enabled` Nullable(UInt8),
  `sort_order` Nullable(Int64),
  `allow_balance_pay` Nullable(UInt8),
  `stripe_price_id` Nullable(String),
  `creem_product_id` Nullable(String),
  `waffo_pancake_product_id` Nullable(String),
  `max_purchase_per_user` Nullable(Int64),
  `upgrade_group` Nullable(String),
  `total_amount` Int64,
  `quota_reset_period` Nullable(String),
  `quota_reset_custom_seconds` Nullable(Int64),
  `created_at` Nullable(Int64),
  `updated_at` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `subscription_pre_consume_records` (
  `id` Int64,
  `request_id` Nullable(String),
  `user_id` Nullable(Int64),
  `user_subscription_id` Nullable(Int64),
  `pre_consumed` Int64,
  `status` Nullable(String),
  `created_at` Nullable(Int64),
  `updated_at` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `tasks` (
  `id` Int64,
  `created_at` Nullable(Int64),
  `updated_at` Nullable(Int64),
  `task_id` Nullable(String),
  `platform` Nullable(String),
  `user_id` Nullable(Int64),
  `group` Nullable(String),
  `channel_id` Nullable(Int64),
  `quota` Nullable(Int64),
  `action` Nullable(String),
  `status` Nullable(String),
  `fail_reason` Nullable(String),
  `submit_time` Nullable(Int64),
  `start_time` Nullable(Int64),
  `finish_time` Nullable(Int64),
  `progress` Nullable(String),
  `properties` Nullable(String),
  `private_data` Nullable(String),
  `data` Nullable(String)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `tokens` (
  `id` Int64,
  `user_id` Nullable(Int64),
  `key` Nullable(String),
  `status` Nullable(Int64),
  `name` Nullable(String),
  `created_time` Nullable(Int64),
  `accessed_time` Nullable(Int64),
  `expired_time` Nullable(Int64),
  `remain_quota` Nullable(Int64),
  `unlimited_quota` Nullable(UInt8),
  `model_limits_enabled` Nullable(UInt8),
  `model_limits` Nullable(String),
  `allow_ips` Nullable(String),
  `used_quota` Nullable(Int64),
  `group` Nullable(String),
  `cross_group_retry` Nullable(UInt8),
  `deleted_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `top_ups` (
  `id` Int64,
  `user_id` Nullable(Int64),
  `amount` Nullable(Int64),
  `money` Nullable(Float64),
  `trade_no` Nullable(String),
  `payment_method` Nullable(String),
  `payment_provider` Nullable(String),
  `create_time` Nullable(Int64),
  `complete_time` Nullable(Int64),
  `status` Nullable(String)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `two_fa_backup_codes` (
  `id` Int64,
  `user_id` Int64,
  `code_hash` String,
  `is_used` Nullable(UInt8),
  `used_at` Nullable(DateTime64(3)),
  `created_at` Nullable(DateTime64(3)),
  `deleted_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `two_fas` (
  `id` Int64,
  `user_id` Int64,
  `secret` String,
  `is_enabled` Nullable(UInt8),
  `failed_attempts` Nullable(Int64),
  `locked_until` Nullable(DateTime64(3)),
  `last_used_at` Nullable(DateTime64(3)),
  `created_at` Nullable(DateTime64(3)),
  `updated_at` Nullable(DateTime64(3)),
  `deleted_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `user_oauth_bindings` (
  `id` Int64,
  `user_id` Int64,
  `provider_id` Int64,
  `provider_user_id` String,
  `created_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `user_subscriptions` (
  `id` Int64,
  `user_id` Nullable(Int64),
  `plan_id` Nullable(Int64),
  `amount_total` Int64,
  `amount_used` Int64,
  `start_time` Nullable(Int64),
  `end_time` Nullable(Int64),
  `status` Nullable(String),
  `source` Nullable(String),
  `last_reset_time` Nullable(Int64),
  `next_reset_time` Nullable(Int64),
  `upgrade_group` Nullable(String),
  `prev_user_group` Nullable(String),
  `created_at` Nullable(Int64),
  `updated_at` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `users` (
  `id` Int64,
  `username` Nullable(String),
  `password` String,
  `display_name` Nullable(String),
  `role` Nullable(Int64),
  `status` Nullable(Int64),
  `email` Nullable(String),
  `github_id` Nullable(String),
  `discord_id` Nullable(String),
  `oidc_id` Nullable(String),
  `wechat_id` Nullable(String),
  `telegram_id` Nullable(String),
  `access_token` Nullable(String),
  `quota` Nullable(Int64),
  `used_quota` Nullable(Int64),
  `request_count` Nullable(Int64),
  `group` Nullable(String),
  `aff_code` Nullable(String),
  `aff_count` Nullable(Int64),
  `aff_quota` Nullable(Int64),
  `aff_history` Nullable(Int64),
  `inviter_id` Nullable(Int64),
  `deleted_at` Nullable(DateTime64(3)),
  `linux_do_id` Nullable(String),
  `setting` Nullable(String),
  `remark` Nullable(String),
  `stripe_customer` Nullable(String),
  `created_at` Nullable(Int64),
  `last_login_at` Nullable(Int64)
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1;

CREATE TABLE IF NOT EXISTS `vendors` (
  `id` Int64,
  `name` String,
  `description` Nullable(String),
  `icon` Nullable(String),
  `status` Nullable(Int64),
  `created_time` Nullable(Int64),
  `updated_time` Nullable(Int64),
  `deleted_at` Nullable(DateTime64(3))
) ENGINE=MergeTree ORDER BY (`id`) SETTINGS enable_block_number_column=1, enable_block_offset_column=1
