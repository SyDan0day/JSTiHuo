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
