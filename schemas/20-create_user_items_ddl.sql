CREATE TABLE user_items (
  user_id STRING(36) NOT NULL,
  item_id STRING(36) NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  CONSTRAINT FK_ItemsID FOREIGN KEY (item_id) REFERENCES items (item_id)
) PRIMARY KEY(user_id, item_id),
  INTERLEAVE IN PARENT users ON DELETE CASCADE
