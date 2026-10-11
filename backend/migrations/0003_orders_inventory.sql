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
