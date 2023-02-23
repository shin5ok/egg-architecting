CREATE TABLE items (
  item_id STRING(36) NOT NULL,
  item_name STRING(64) NOT NULL,
  price INT64 NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
) PRIMARY KEY(item_id)
