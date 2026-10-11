-- DESIGN_DECISION: MySQL 8.4 / InnoDB; specification only, not applied.

SET NAMES utf8mb4;

SET time_zone = '+00:00';

CREATE TABLE `admin_users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `login_name` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `enabled` BOOLEAN NOT NULL DEFAULT 1,
  `force_password_change` BOOLEAN NOT NULL DEFAULT 1,
  `auth_version` BIGINT NOT NULL DEFAULT 1,
  `last_login_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_admin_users_0` (`login_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `roles` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `name` VARCHAR(64) NOT NULL,
  `enabled` BOOLEAN NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_roles_0` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `permissions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `permission_key` VARCHAR(80) NOT NULL,
  `description` VARCHAR(200) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_permissions_0` (`permission_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `admin_user_roles` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `admin_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_admin_user_roles_0` (`admin_id`, `role_id`),
  KEY `k_admin_user_roles_1` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `role_permissions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `permission_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_role_permissions_0` (`role_id`, `permission_id`),
  KEY `k_role_permissions_1` (`permission_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `categories` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `name` VARCHAR(80) NOT NULL,
  `sort_order` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_categories_0` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `files` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `storage_key` VARCHAR(200) NOT NULL,
  `kind` VARCHAR(24) NOT NULL DEFAULT 'IMAGE',
  `mime_type` VARCHAR(100) NOT NULL,
  `size_bytes` BIGINT NOT NULL DEFAULT 0,
  `sha256` BINARY(32) NOT NULL,
  `state` VARCHAR(24) NOT NULL DEFAULT 'STAGED',
  `owner_admin_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `key_version` INT UNSIGNED NULL DEFAULT NULL,
  `expires_at` DATETIME(6) NULL DEFAULT NULL,
  `deleted_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_files_0` (`storage_key`),
  KEY `k_files_1` (`state`, `expires_at`),
  KEY `k_files_2` (`owner_admin_id`),
  CHECK (`kind` IN ('IMAGE','IMPORT','CREDENTIAL_EXPORT','REPORT')),
  CHECK (`state` IN ('STAGED','READY','EXPIRED','DELETED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `products` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `code` VARCHAR(64) NOT NULL,
  `name` VARCHAR(120) NOT NULL,
  `category_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `attributes` JSON NOT NULL,
  `description_html` MEDIUMTEXT NOT NULL,
  `external_spec_code` VARCHAR(64) NULL DEFAULT NULL,
  `external_spec_name` VARCHAR(120) NULL DEFAULT NULL,
  `redeem_from` DATETIME(6) NULL DEFAULT NULL,
  `redeem_until` DATETIME(6) NULL DEFAULT NULL,
  `excluded_region_codes` JSON NOT NULL,
  `sort_order` BIGINT NOT NULL DEFAULT 0,
  `status` VARCHAR(24) NOT NULL DEFAULT 'DRAFT',
  `deleted_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_products_0` (`code`),
  KEY `k_products_1` (`status`, `sort_order`, `id`),
  KEY `k_products_2` (`category_id`),
  CHECK (`status` IN ('DRAFT','ENABLED','DISABLED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `product_media` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `product_id` BIGINT UNSIGNED NOT NULL,
  `file_id` BIGINT UNSIGNED NOT NULL,
  `position` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_product_media_0` (`product_id`, `position`),
  KEY `k_product_media_1` (`file_id`),
  CONSTRAINT `ck_product_media_0` CHECK (position BETWEEN 0 AND 8)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `voucher_types` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `name` VARCHAR(80) NOT NULL,
  `revision` INT UNSIGNED NOT NULL DEFAULT 1,
  `selection_mode` VARCHAR(24) NOT NULL DEFAULT 'ONE_OF',
  `selection_count` INT UNSIGNED NOT NULL DEFAULT 1,
  `enabled` BOOLEAN NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_voucher_types_0` (`name`, `revision`),
  CONSTRAINT `ck_voucher_types_0` CHECK (selection_count BETWEEN 1 AND 99),
  CONSTRAINT `ck_voucher_types_1` CHECK (selection_mode <> 'ONE_OF' OR selection_count = 1),
  CHECK (`selection_mode` IN ('ONE_OF','N_OF'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `voucher_batches` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `batch_no` VARCHAR(40) NOT NULL,
  `type_id` BIGINT UNSIGNED NOT NULL,
  `label` VARCHAR(100) NOT NULL,
  `policy_snapshot` JSON NOT NULL,
  `quantity` INT UNSIGNED NOT NULL,
  `code_prefix` VARCHAR(4) NOT NULL,
  `code_length` INT UNSIGNED NOT NULL DEFAULT 16,
  `code_mode` VARCHAR(24) NOT NULL DEFAULT 'RANDOM',
  `start_serial` VARCHAR(24) NULL DEFAULT NULL,
  `secret_length` INT UNSIGNED NOT NULL DEFAULT 12,
  `secret_format` VARCHAR(24) NOT NULL DEFAULT 'MIXED',
  `valid_from` DATETIME(6) NOT NULL,
  `valid_until` DATETIME(6) NOT NULL,
  `activate_on_seal` BOOLEAN NOT NULL DEFAULT 0,
  `state` VARCHAR(24) NOT NULL DEFAULT 'DRAFT',
  `export_file_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `lease_until` DATETIME(6) NULL DEFAULT NULL,
  `generation_epoch` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `last_error_code` VARCHAR(80) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_voucher_batches_0` (`batch_no`),
  KEY `k_voucher_batches_1` (`state`, `lease_until`),
  KEY `k_voucher_batches_2` (`type_id`),
  KEY `k_voucher_batches_3` (`export_file_id`),
  KEY `k_voucher_batches_4` (`created_by`),
  CONSTRAINT `ck_voucher_batches_0` CHECK (quantity BETWEEN 1 AND 1000),
  CONSTRAINT `ck_voucher_batches_1` CHECK (valid_until > valid_from),
  CONSTRAINT `ck_voucher_batches_2` CHECK (code_length BETWEEN 12 AND 24),
  CONSTRAINT `ck_voucher_batches_3` CHECK (secret_length IN (6,8,9,10,11,12,16,24,32)),
  CHECK (`code_mode` IN ('SEQUENTIAL','RANDOM')),
  CHECK (`secret_format` IN ('DIGITS','LETTERS','MIXED')),
  CHECK (`state` IN ('DRAFT','GENERATING','GENERATED','SEALED','FAILED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `vouchers` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `batch_id` BIGINT UNSIGNED NOT NULL,
  `credential_file_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `code` VARCHAR(32) NOT NULL,
  `secret_hash` VARCHAR(255) NOT NULL,
  `credential_version` INT UNSIGNED NOT NULL DEFAULT 1,
  `active` BOOLEAN NOT NULL DEFAULT 0,
  `credential_state` VARCHAR(24) NOT NULL DEFAULT 'EXPORTABLE',
  `valid_from` DATETIME(6) NOT NULL,
  `valid_until` DATETIME(6) NOT NULL,
  `redeemed_at` DATETIME(6) NULL DEFAULT NULL,
  `deleted_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_vouchers_0` (`code`),
  KEY `k_vouchers_1` (`batch_id`, `id`),
  KEY `k_vouchers_2` (`active`, `redeemed_at`),
  KEY `k_vouchers_3` (`credential_file_id`),
  CONSTRAINT `ck_vouchers_0` CHECK (valid_until > valid_from),
  CHECK (`credential_state` IN ('EXPORTABLE','SEALED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `voucher_products` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `voucher_id` BIGINT UNSIGNED NOT NULL,
  `product_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_voucher_products_0` (`voucher_id`, `product_id`),
  KEY `k_voucher_products_1` (`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `voucher_redemptions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `voucher_id` BIGINT UNSIGNED NOT NULL,
  `policy_snapshot` JSON NOT NULL,
  `redeemed_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_voucher_redemptions_0` (`voucher_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `shipping_calendars` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `ship_date` DATE NOT NULL,
  `capacity` INT UNSIGNED NOT NULL DEFAULT 100,
  `used_slots` INT UNSIGNED NOT NULL DEFAULT 0,
  `closed` BOOLEAN NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_shipping_calendars_0` (`ship_date`),
  CONSTRAINT `ck_shipping_calendars_0` CHECK (used_slots <= capacity)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `order_no` VARCHAR(40) NOT NULL,
  `redemption_id` BIGINT UNSIGNED NOT NULL,
  `status` VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  `fulfillment_mode` VARCHAR(24) NOT NULL DEFAULT 'DELIVERY',
  `calendar_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `recipient_cipher` BLOB NOT NULL,
  `pii_key_version` INT UNSIGNED NOT NULL,
  `phone_lookup` BINARY(32) NOT NULL,
  `customer_note` VARCHAR(200) NOT NULL,
  `admin_note` VARCHAR(1000) NOT NULL,
  `shipped_at` DATETIME(6) NULL DEFAULT NULL,
  `completed_at` DATETIME(6) NULL DEFAULT NULL,
  `cancelled_at` DATETIME(6) NULL DEFAULT NULL,
  `cancel_reason` VARCHAR(200) NULL DEFAULT NULL,
  `deleted_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_orders_0` (`order_no`),
  UNIQUE KEY `k_orders_1` (`redemption_id`),
  KEY `k_orders_2` (`status`, `calendar_id`, `id`),
  KEY `k_orders_3` (`phone_lookup`),
  KEY `k_orders_4` (`deleted_at`),
  KEY `k_orders_5` (`calendar_id`),
  CHECK (`status` IN ('PENDING','SHIPPED','COMPLETED','CANCELLED')),
  CHECK (`fulfillment_mode` IN ('DELIVERY','PICKUP'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `order_items` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `product_id` BIGINT UNSIGNED NOT NULL,
  `quantity` INT UNSIGNED NOT NULL DEFAULT 1,
  `product_snapshot` JSON NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_order_items_0` (`order_id`, `product_id`),
  KEY `k_order_items_1` (`product_id`),
  CONSTRAINT `ck_order_items_0` CHECK (quantity = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `inventory_balances` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `product_id` BIGINT UNSIGNED NOT NULL,
  `on_hand` BIGINT NOT NULL DEFAULT 0,
  `reserved` BIGINT NOT NULL DEFAULT 0,
  `warning_level` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_inventory_balances_0` (`product_id`),
  CONSTRAINT `ck_inventory_balances_0` CHECK (on_hand >= 0),
  CONSTRAINT `ck_inventory_balances_1` CHECK (reserved >= 0),
  CONSTRAINT `ck_inventory_balances_2` CHECK (reserved <= on_hand),
  CONSTRAINT `ck_inventory_balances_3` CHECK (warning_level >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `inventory_documents` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `document_no` VARCHAR(40) NOT NULL,
  `direction` VARCHAR(24) NOT NULL DEFAULT 'IN',
  `stock_kind` VARCHAR(24) NOT NULL DEFAULT 'PHYSICAL',
  `state` VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  `note` VARCHAR(500) NOT NULL,
  `proof_file_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `reviewed_by` BIGINT UNSIGNED NULL DEFAULT NULL,
  `reviewed_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_inventory_documents_0` (`document_no`),
  KEY `k_inventory_documents_1` (`state`, `id`),
  KEY `k_inventory_documents_2` (`proof_file_id`),
  KEY `k_inventory_documents_3` (`created_by`),
  KEY `k_inventory_documents_4` (`reviewed_by`),
  CHECK (`direction` IN ('IN','OUT')),
  CHECK (`stock_kind` IN ('PHYSICAL','VIRTUAL')),
  CHECK (`state` IN ('PENDING','APPROVED','REJECTED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `inventory_document_items` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `document_id` BIGINT UNSIGNED NOT NULL,
  `product_id` BIGINT UNSIGNED NOT NULL,
  `quantity` INT UNSIGNED NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_inventory_document_items_0` (`document_id`, `product_id`),
  KEY `k_inventory_document_items_1` (`product_id`),
  CONSTRAINT `ck_inventory_document_items_0` CHECK (quantity > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `inventory_reservations` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `order_item_id` BIGINT UNSIGNED NOT NULL,
  `product_id` BIGINT UNSIGNED NOT NULL,
  `quantity` INT UNSIGNED NOT NULL,
  `state` VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_inventory_reservations_0` (`order_item_id`),
  KEY `k_inventory_reservations_1` (`product_id`, `state`),
  CONSTRAINT `ck_inventory_reservations_0` CHECK (quantity > 0),
  CHECK (`state` IN ('ACTIVE','CONSUMED','RELEASED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `carriers` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `code` VARCHAR(40) NOT NULL,
  `name` VARCHAR(80) NOT NULL,
  `alias` VARCHAR(80) NOT NULL,
  `kdniao_code` VARCHAR(40) NULL DEFAULT NULL,
  `kuaidi100_code` VARCHAR(40) NULL DEFAULT NULL,
  `wdt_code` VARCHAR(40) NULL DEFAULT NULL,
  `provider` VARCHAR(24) NOT NULL DEFAULT 'MANUAL',
  `enabled` BOOLEAN NOT NULL DEFAULT 1,
  `secret_ref` VARCHAR(120) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_carriers_0` (`code`),
  CHECK (`provider` IN ('MANUAL','MOCK','KDNIAO','KUAIDI100'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `after_sales` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `kind` VARCHAR(24) NOT NULL DEFAULT 'COMPLAINT',
  `state` VARCHAR(24) NOT NULL DEFAULT 'OPEN',
  `reason` VARCHAR(500) NOT NULL,
  `items` JSON NOT NULL,
  `resolution` VARCHAR(1000) NOT NULL,
  `handled_by` BIGINT UNSIGNED NULL DEFAULT NULL,
  `closed_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `k_after_sales_0` (`order_id`, `state`),
  KEY `k_after_sales_1` (`handled_by`),
  CHECK (`kind` IN ('COMPLAINT','REPLACEMENT')),
  CHECK (`state` IN ('OPEN','APPROVED','REJECTED','CLOSED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `shipments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `after_sale_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `shipment_key` VARCHAR(100) NOT NULL,
  `kind` VARCHAR(24) NOT NULL DEFAULT 'PRIMARY',
  `carrier_id` BIGINT UNSIGNED NOT NULL,
  `tracking_no` VARCHAR(80) NOT NULL,
  `status` VARCHAR(24) NOT NULL DEFAULT 'SHIPPED',
  `shipped_at` DATETIME(6) NOT NULL,
  `delivered_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_shipments_0` (`shipment_key`),
  UNIQUE KEY `k_shipments_1` (`carrier_id`, `tracking_no`),
  KEY `k_shipments_2` (`order_id`),
  KEY `k_shipments_3` (`after_sale_id`),
  CHECK (`kind` IN ('PRIMARY','REPLACEMENT')),
  CHECK (`status` IN ('SHIPPED','IN_TRANSIT','DELIVERED','EXCEPTION'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `shipment_events` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `shipment_id` BIGINT UNSIGNED NOT NULL,
  `provider_event_id` VARCHAR(120) NOT NULL,
  `event_type` VARCHAR(40) NOT NULL,
  `description` VARCHAR(300) NOT NULL,
  `occurred_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_shipment_events_0` (`shipment_id`, `provider_event_id`),
  KEY `k_shipment_events_1` (`shipment_id`, `occurred_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

CREATE TABLE `inventory_transactions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
  `product_id` BIGINT UNSIGNED NOT NULL,
  `on_hand_delta` BIGINT NOT NULL DEFAULT 0,
  `reserved_delta` BIGINT NOT NULL DEFAULT 0,
  `event_key` VARCHAR(120) NOT NULL,
  `order_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `document_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `shipment_id` BIGINT UNSIGNED NULL DEFAULT NULL,
  `reason` VARCHAR(200) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `k_inventory_transactions_0` (`event_key`),
  KEY `k_inventory_transactions_1` (`product_id`, `id`),
  KEY `k_inventory_transactions_2` (`order_id`),
  KEY `k_inventory_transactions_3` (`document_id`),
  KEY `k_inventory_transactions_4` (`shipment_id`),
  CONSTRAINT `ck_inventory_transactions_0` CHECK (on_hand_delta <> 0 OR reserved_delta <> 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_as_cs;

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

ALTER TABLE `admin_user_roles` ADD CONSTRAINT `fk_admin_user_roles_admin_id` FOREIGN KEY (`admin_id`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `admin_user_roles` ADD CONSTRAINT `fk_admin_user_roles_role_id` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `role_permissions` ADD CONSTRAINT `fk_role_permissions_role_id` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `role_permissions` ADD CONSTRAINT `fk_role_permissions_permission_id` FOREIGN KEY (`permission_id`) REFERENCES `permissions` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `files` ADD CONSTRAINT `fk_files_owner_admin_id` FOREIGN KEY (`owner_admin_id`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `products` ADD CONSTRAINT `fk_products_category_id` FOREIGN KEY (`category_id`) REFERENCES `categories` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `product_media` ADD CONSTRAINT `fk_product_media_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `product_media` ADD CONSTRAINT `fk_product_media_file_id` FOREIGN KEY (`file_id`) REFERENCES `files` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `voucher_batches` ADD CONSTRAINT `fk_voucher_batches_type_id` FOREIGN KEY (`type_id`) REFERENCES `voucher_types` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `voucher_batches` ADD CONSTRAINT `fk_voucher_batches_export_file_id` FOREIGN KEY (`export_file_id`) REFERENCES `files` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `voucher_batches` ADD CONSTRAINT `fk_voucher_batches_created_by` FOREIGN KEY (`created_by`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `vouchers` ADD CONSTRAINT `fk_vouchers_batch_id` FOREIGN KEY (`batch_id`) REFERENCES `voucher_batches` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `vouchers` ADD CONSTRAINT `fk_vouchers_credential_file_id` FOREIGN KEY (`credential_file_id`) REFERENCES `files` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `voucher_products` ADD CONSTRAINT `fk_voucher_products_voucher_id` FOREIGN KEY (`voucher_id`) REFERENCES `vouchers` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `voucher_products` ADD CONSTRAINT `fk_voucher_products_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `voucher_redemptions` ADD CONSTRAINT `fk_voucher_redemptions_voucher_id` FOREIGN KEY (`voucher_id`) REFERENCES `vouchers` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `orders` ADD CONSTRAINT `fk_orders_redemption_id` FOREIGN KEY (`redemption_id`) REFERENCES `voucher_redemptions` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `orders` ADD CONSTRAINT `fk_orders_calendar_id` FOREIGN KEY (`calendar_id`) REFERENCES `shipping_calendars` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `order_items` ADD CONSTRAINT `fk_order_items_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `order_items` ADD CONSTRAINT `fk_order_items_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_balances` ADD CONSTRAINT `fk_inventory_balances_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_documents` ADD CONSTRAINT `fk_inventory_documents_proof_file_id` FOREIGN KEY (`proof_file_id`) REFERENCES `files` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_documents` ADD CONSTRAINT `fk_inventory_documents_created_by` FOREIGN KEY (`created_by`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_documents` ADD CONSTRAINT `fk_inventory_documents_reviewed_by` FOREIGN KEY (`reviewed_by`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_document_items` ADD CONSTRAINT `fk_inventory_document_items_document_id` FOREIGN KEY (`document_id`) REFERENCES `inventory_documents` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_document_items` ADD CONSTRAINT `fk_inventory_document_items_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_reservations` ADD CONSTRAINT `fk_inventory_reservations_order_item_id` FOREIGN KEY (`order_item_id`) REFERENCES `order_items` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_reservations` ADD CONSTRAINT `fk_inventory_reservations_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `after_sales` ADD CONSTRAINT `fk_after_sales_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `after_sales` ADD CONSTRAINT `fk_after_sales_handled_by` FOREIGN KEY (`handled_by`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `shipments` ADD CONSTRAINT `fk_shipments_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `shipments` ADD CONSTRAINT `fk_shipments_after_sale_id` FOREIGN KEY (`after_sale_id`) REFERENCES `after_sales` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `shipments` ADD CONSTRAINT `fk_shipments_carrier_id` FOREIGN KEY (`carrier_id`) REFERENCES `carriers` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `shipment_events` ADD CONSTRAINT `fk_shipment_events_shipment_id` FOREIGN KEY (`shipment_id`) REFERENCES `shipments` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_transactions` ADD CONSTRAINT `fk_inventory_transactions_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_transactions` ADD CONSTRAINT `fk_inventory_transactions_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_transactions` ADD CONSTRAINT `fk_inventory_transactions_document_id` FOREIGN KEY (`document_id`) REFERENCES `inventory_documents` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `inventory_transactions` ADD CONSTRAINT `fk_inventory_transactions_shipment_id` FOREIGN KEY (`shipment_id`) REFERENCES `shipments` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `sessions` ADD CONSTRAINT `fk_sessions_admin_id` FOREIGN KEY (`admin_id`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `sessions` ADD CONSTRAINT `fk_sessions_voucher_id` FOREIGN KEY (`voucher_id`) REFERENCES `vouchers` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `sessions` ADD CONSTRAINT `fk_sessions_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `challenges` ADD CONSTRAINT `fk_challenges_session_id` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `notifications` ADD CONSTRAINT `fk_notifications_outbox_id` FOREIGN KEY (`outbox_id`) REFERENCES `outbox_events` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `notifications` ADD CONSTRAINT `fk_notifications_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `integration_jobs` ADD CONSTRAINT `fk_integration_jobs_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `integration_jobs` ADD CONSTRAINT `fk_integration_jobs_file_id` FOREIGN KEY (`file_id`) REFERENCES `files` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `integration_jobs` ADD CONSTRAINT `fk_integration_jobs_result_file_id` FOREIGN KEY (`result_file_id`) REFERENCES `files` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `feedback` ADD CONSTRAINT `fk_feedback_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `system_configs` ADD CONSTRAINT `fk_system_configs_updated_by` FOREIGN KEY (`updated_by`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE `audit_logs` ADD CONSTRAINT `fk_audit_logs_actor_admin_id` FOREIGN KEY (`actor_admin_id`) REFERENCES `admin_users` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT;
