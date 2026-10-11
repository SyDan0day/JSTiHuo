CREATE TABLE `sessions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `token_hash` BINARY(32) NOT NULL,
  `kind` VARCHAR(24) NOT NULL DEFAULT 'PREAUTH',
  `scope` VARCHAR(24) NOT NULL DEFAULT 'READ',
  `admin_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `voucher_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `order_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `subject_version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `csrf_hash` BINARY(32) NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  `last_seen_at` DATETIME(6) NOT NULL,
  `reauth_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_sessions_0` (`token_hash`),
  KEY `k_sessions_1` (`expires_at`),
  KEY `k_sessions_2` (`admin_id`),
  KEY `k_sessions_3` (`voucher_id`),
  KEY `k_sessions_4` (`order_id`),
  CHECK (`kind` IN ('ADMIN','VOUCHER','ORDER','PREAUTH')),
  CHECK (`scope` IN ('READ','WRITE','AUTH_ONLY'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `challenges` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `session_id` BIGINT UNSIGNED NOT NULL,
  `purpose` VARCHAR(30) NOT NULL,
  `answer_hash` BINARY(32) NOT NULL,
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  `expires_at` DATETIME(6) NOT NULL,
  `solved_at` DATETIME(6) NULL DEFAULT NULL,
  `consumed_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `k_challenges_0` (`session_id`, `purpose`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `idempotency_keys` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `scope` VARCHAR(120) NOT NULL,
  `request_key` VARCHAR(64) NOT NULL,
  `request_hash` BINARY(32) NOT NULL,
  `response_status` SMALLINT UNSIGNED NULL DEFAULT NULL,
  `result_ref` JSON NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_idempotency_keys_0` (`scope`, `request_key`),
  KEY `k_idempotency_keys_1` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `outbox_events` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `event_key` VARCHAR(140) NOT NULL,
  `event_type` VARCHAR(80) NOT NULL,
  `payload` JSON NOT NULL,
  `state` VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_attempt_at` DATETIME(6) NOT NULL,
  `lease_until` DATETIME(6) NULL DEFAULT NULL,
  `last_error_code` VARCHAR(80) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_outbox_events_0` (`event_key`),
  KEY `k_outbox_events_1` (`state`, `next_attempt_at`),
  CHECK (`state` IN ('PENDING','PROCESSING','DONE','DEAD'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `notifications` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `outbox_id` BIGINT UNSIGNED NOT NULL,
  `order_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `channel` VARCHAR(24) NOT NULL DEFAULT 'MOCK',
  `template_key` VARCHAR(80) NOT NULL,
  `state` VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  `provider_message_id` VARCHAR(120) NULL DEFAULT NULL,
  `last_error_code` VARCHAR(80) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_notifications_0` (`outbox_id`, `channel`),
  KEY `k_notifications_1` (`order_id`),
  CHECK (`channel` IN ('SMS','EMAIL','WECHAT','MOCK')),
  CHECK (`state` IN ('PENDING','SENT','FAILED','UNKNOWN','SUPPRESSED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `integration_jobs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `job_key` VARCHAR(120) NOT NULL,
  `kind` VARCHAR(24) NOT NULL DEFAULT 'LABEL',
  `state` VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  `order_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `file_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `result_file_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `options` JSON NOT NULL,
  `result_summary` JSON NOT NULL,
  `lease_until` DATETIME(6) NULL DEFAULT NULL,
  `provider_request_id` VARCHAR(120) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_integration_jobs_0` (`job_key`),
  KEY `k_integration_jobs_1` (`state`, `id`),
  KEY `k_integration_jobs_2` (`order_id`),
  KEY `k_integration_jobs_3` (`file_id`),
  KEY `k_integration_jobs_4` (`result_file_id`),
  CHECK (`kind` IN ('LABEL','TRACKING','IMPORT_VOUCHERS','IMPORT_SHIPMENTS','EXPORT_REPORT')),
  CHECK (`state` IN ('PENDING','RUNNING','READY','FAILED','UNKNOWN'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `feedback` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `order_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `contact_cipher` BLOB NOT NULL,
  `pii_key_version` INT UNSIGNED NOT NULL,
  `message` VARCHAR(200) NOT NULL,
  `state` VARCHAR(24) NOT NULL DEFAULT 'OPEN',
  `resolution` VARCHAR(500) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `k_feedback_0` (`state`, `id`),
  KEY `k_feedback_1` (`order_id`),
  CHECK (`state` IN ('OPEN','RESOLVED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `content_pages` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `slug` VARCHAR(64) NOT NULL,
  `title` VARCHAR(120) NOT NULL,
  `body_html` MEDIUMTEXT NOT NULL,
  `kind` VARCHAR(24) NOT NULL DEFAULT 'PAGE',
  `published` BOOLEAN NOT NULL DEFAULT 0,
  `sort_order` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_content_pages_0` (`slug`),
  KEY `k_content_pages_1` (`published`, `sort_order`),
  CHECK (`kind` IN ('PAGE','ANNOUNCEMENT','BANNER'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `system_configs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `config_key` VARCHAR(80) NOT NULL,
  `value` JSON NOT NULL,
  `updated_by` BIGINT UNSIGNED NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_system_configs_0` (`config_key`),
  KEY `k_system_configs_1` (`updated_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `audit_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `actor_admin_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `actor_kind` VARCHAR(20) NOT NULL,
  `action` VARCHAR(100) NOT NULL,
  `entity_type` VARCHAR(50) NOT NULL,
  `entity_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `request_id` VARCHAR(64) NOT NULL,
  `diff` JSON NOT NULL,
  `result` VARCHAR(40) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `k_audit_logs_0` (`entity_type`, `entity_id`, `id`),
  KEY `k_audit_logs_1` (`actor_admin_id`, `id`),
  KEY `k_audit_logs_2` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `rate_limits` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `bucket_hash` BINARY(32) NOT NULL,
  `window_end` DATETIME(6) NOT NULL,
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_rate_limits_0` (`bucket_hash`),
  KEY `k_rate_limits_1` (`window_end`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;
